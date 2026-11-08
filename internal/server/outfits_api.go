package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"net/http"
	"strconv"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/style"
)

// ---- saved outfits ----

func (s *Server) decorateOutfit(o *store.Outfit) *store.Outfit {
	o.PhotoURL, _ = photoURL(o.PhotoID)
	return o
}

type outfitInput struct {
	Name      *string   `json:"name"`
	ItemIDs   *[]int64  `json:"item_ids"`
	Style     *string   `json:"style"`
	Occasion  *string   `json:"occasion"`
	Seasons   *[]string `json:"seasons"`
	Rating    *int      `json:"rating"`
	Favourite *bool     `json:"favourite"`
	Notes     *string   `json:"notes"`
	PhotoID   *int64    `json:"photo_id"`
	MadeBy    *string   `json:"made_by"`
}

func (s *Server) applyOutfit(ctx context.Context, uid int64, in outfitInput, o *store.Outfit) string {
	if in.Name != nil {
		o.Name = clip(strings.TrimSpace(*in.Name), 120)
	}
	if in.ItemIDs != nil {
		own, err := s.store.OwnIDs(ctx, uid, *in.ItemIDs)
		if err != nil || len(own) == 0 {
			return "pick at least one of your pieces"
		}
		if len(own) > 12 {
			return "an outfit has at most 12 pieces"
		}
		o.ItemIDs = own
	}
	if in.Style != nil {
		if _, ok := style.StyleByID(*in.Style); !ok && *in.Style != "" {
			return "unknown style"
		}
		o.Style = *in.Style
	}
	if in.Occasion != nil {
		if *in.Occasion != "" && style.OccasionByID(*in.Occasion).ID != *in.Occasion {
			return "unknown occasion"
		}
		o.Occasion = *in.Occasion
	}
	if in.Seasons != nil {
		ss := []string{}
		for _, v := range *in.Seasons {
			if oneOf(v, style.Seasons) && !oneOf(v, ss) {
				ss = append(ss, v)
			}
		}
		o.Seasons = ss
	}
	if in.Rating != nil {
		if *in.Rating < 0 || *in.Rating > 5 {
			return "the rating is 0 to 5"
		}
		o.Rating = *in.Rating
	}
	if in.Favourite != nil {
		o.Favourite = *in.Favourite
	}
	if in.Notes != nil {
		o.Notes = clip(strings.TrimSpace(*in.Notes), 2000)
	}
	if in.PhotoID != nil {
		id, ok := s.photoMine(ctx, uid, in.PhotoID)
		if !ok {
			return "unknown photo"
		}
		o.PhotoID = id
	}
	if len(o.ItemIDs) == 0 {
		return "pick at least one of your pieces"
	}
	if o.Name == "" {
		o.Name = s.outfitName(ctx, uid, o.ItemIDs)
	}
	return ""
}

// outfitName makes a name from the main pieces ("Navy blazer, white shirt").
func (s *Server) outfitName(ctx context.Context, uid int64, ids []int64) string {
	m, _ := s.store.ItemsByIDs(ctx, uid, ids)
	order := map[string]int{"outerwear": 0, "dress": 1, "knit": 2, "top": 3, "bottom": 4, "shoes": 5}
	var names []string
	for pass := 0; pass < 6 && len(names) < 2; pass++ {
		for _, id := range ids {
			if it := m[id]; it != nil && order[it.Category] == pass && (pass > 0 || it.Category == "outerwear") {
				names = append(names, strings.ToLower(it.Name))
			}
		}
	}
	if len(names) == 0 {
		return "Outfit"
	}
	n := strings.Join(names, ", ")
	return strings.ToUpper(n[:1]) + n[1:]
}

func (s *Server) handleListOutfits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	list, err := s.store.ListOutfits(ctx, uid)
	if err != nil {
		s.fail(w, err, "outfits")
		return
	}
	ids := []int64{}
	for _, o := range list {
		s.decorateOutfit(o)
		ids = append(ids, o.ItemIDs...)
	}
	items, _ := s.store.ItemsByIDs(ctx, uid, ids)
	for _, it := range items {
		decorateItem(it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"outfits": list, "items": items})
}

func (s *Server) ownOutfit(w http.ResponseWriter, r *http.Request, write bool) *store.Outfit {
	ctx := r.Context()
	o, err := s.store.GetOutfit(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "outfit")
		return nil
	}
	if app := linkApp(ctx); write && app != "" && o.MadeBy != app {
		writeErr(w, http.StatusNotFound, "not found")
		return nil
	}
	return o
}

func (s *Server) handleGetOutfit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	o := s.ownOutfit(w, r, false)
	if o == nil {
		return
	}
	s.decorateOutfit(o)
	items, _ := s.store.ItemsByIDs(ctx, uid, o.ItemIDs)
	for _, it := range items {
		decorateItem(it)
	}
	pieces, _ := s.pieces(ctx, uid, today())
	var ps []style.Piece
	for _, p := range pieces {
		if hasInt(o.ItemIDs, p.ID) {
			ps = append(ps, p)
		}
	}
	c := s.baseContext(ctx, uid, o.Style)
	c.Occasion = firstNonEmpty(o.Occasion, "everyday")
	c.Season = ""
	writeJSON(w, http.StatusOK, map[string]any{"outfit": o, "items": items, "rating": style.Rate(ps, c), "share": s.shareURL(uid, o.ItemIDs)})
}

func (s *Server) handleCreateOutfit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in outfitInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	o := &store.Outfit{UserID: uid, MadeBy: "me", Seasons: []string{}}
	if app := linkApp(ctx); app != "" {
		o.MadeBy = app
	} else if in.MadeBy != nil && oneOf(*in.MadeBy, []string{"me", "ideas", "ai"}) {
		o.MadeBy = *in.MadeBy
	}
	if msg := s.applyOutfit(ctx, uid, in, o); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.CreateOutfit(ctx, o); err != nil {
		s.fail(w, err, "save outfit")
		return
	}
	s.publish(ctx, uid, "wardrobe.outfit_saved", "outfit:"+itoa(o.ID), map[string]any{"id": o.ID, "name": o.Name, "items": len(o.ItemIDs), "url": s.link("/outfits/" + itoa(o.ID))})
	o, _ = s.store.GetOutfit(ctx, uid, o.ID)
	writeJSON(w, http.StatusCreated, s.decorateOutfit(o))
}

func (s *Server) handleUpdateOutfit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	o := s.ownOutfit(w, r, true)
	if o == nil {
		return
	}
	var in outfitInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	old := o.PhotoID
	if msg := s.applyOutfit(ctx, uid, in, o); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.UpdateOutfit(ctx, o); err != nil {
		s.fail(w, err, "save outfit")
		return
	}
	if old != nil && (o.PhotoID == nil || *old != *o.PhotoID) {
		_ = s.store.DeletePhotoIfUnused(ctx, uid, *old)
	}
	o, _ = s.store.GetOutfit(ctx, uid, o.ID)
	writeJSON(w, http.StatusOK, s.decorateOutfit(o))
}

func (s *Server) handleDeleteOutfit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	o := s.ownOutfit(w, r, true)
	if o == nil {
		return
	}
	if err := s.store.DeleteOutfit(ctx, actorID(ctx), o.ID); err != nil {
		s.fail(w, err, "delete outfit")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// ---- what goes together ----

type suggestInput struct {
	Date     string   `json:"date"`
	Occasion string   `json:"occasion"`
	Style    string   `json:"style"`
	Include  int64    `json:"include"`
	Exclude  []int64  `json:"exclude"`
	Shuffle  int64    `json:"shuffle"`
	N        int      `json:"n"`
	TMax     *float64 `json:"t_max"` // instead of the forecast
	TMin     *float64 `json:"t_min"`
	Rain     *bool    `json:"rain"`
	Season   string   `json:"season"`
}

type suggestOut struct {
	Suggestions []style.Score         `json:"suggestions"`
	Items       map[int64]*store.Item `json:"items"`
	Weather     *dayWeather           `json:"weather"`
	WeatherWhy  string                `json:"weather_why,omitempty"`
	Occasion    style.Occasion        `json:"occasion"`
	Style       string                `json:"style"`
	Feel        float64               `json:"feel"`
	Empty       string                `json:"empty,omitempty"`
}

// suggest finds outfits for a day; it is what Today, the generator and
// Pocket AI all use.
func (s *Server) suggest(ctx context.Context, uid int64, in suggestInput) suggestOut {
	c := s.clients(ctx, uid)
	date := firstNonEmpty(in.Date, today())
	sc, w, why := s.dayContext(ctx, c, date, in.Occasion, in.Style)
	if in.TMax != nil || in.TMin != nil {
		sc.TMax, sc.TMin, w = in.TMax, in.TMin, nil
	}
	if in.Rain != nil {
		sc.Rain = *in.Rain
	}
	if oneOf(in.Season, style.Seasons) {
		sc.Season = in.Season
	}
	sc.Include, sc.Exclude, sc.Shuffle = in.Include, in.Exclude, in.Shuffle
	pieces, items := s.pieces(ctx, uid, date)
	n := in.N
	if n <= 0 || n > 12 {
		n = 6
	}
	out := suggestOut{Items: map[int64]*store.Item{}, Weather: w, WeatherWhy: why, Occasion: style.OccasionByID(sc.Occasion), Style: sc.Style, Suggestions: []style.Score{}}
	out.Feel, _ = sc.Feel()
	if len(pieces) < 3 {
		out.Empty = "Add a few pieces first: a top, a bottom and shoes are enough for a first outfit."
		return out
	}
	out.Suggestions = style.Generate(pieces, sc, n)
	if out.Suggestions == nil {
		out.Suggestions = []style.Score{}
	}
	if len(out.Suggestions) == 0 {
		out.Empty = "Nothing complete comes out: add shoes, a top and a bottom (or a dress) that are not in the laundry."
	}
	for _, sg := range out.Suggestions {
		for _, id := range sg.Items {
			if it := items[id]; it != nil {
				out.Items[id] = decorateItem(it)
			}
		}
	}
	return out
}

func (s *Server) handleSuggest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in suggestInput
	if r.Method == http.MethodGet {
		q := r.URL.Query()
		in = suggestInput{Date: q.Get("date"), Occasion: q.Get("occasion"), Style: q.Get("style"), N: queryInt(r, "n"), Shuffle: int64(queryInt(r, "shuffle")), Include: int64(queryInt(r, "include"))}
	} else if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !validDate(in.Date) {
		writeErr(w, http.StatusBadRequest, "the day is YYYY-MM-DD")
		return
	}
	writeJSON(w, http.StatusOK, s.suggest(ctx, actorID(ctx), in))
}

// handleRate scores pieces put together by hand (the outfit maker).
func (s *Server) handleRate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		ItemIDs  []int64 `json:"item_ids"`
		Occasion string  `json:"occasion"`
		Style    string  `json:"style"`
		Date     string  `json:"date"`
		Weather  bool    `json:"weather"` // dress for that day's forecast
	}
	if err := decode(r, &in); err != nil || !validDate(in.Date) {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	date := firstNonEmpty(in.Date, today())
	var sc style.Context
	var wx *dayWeather
	if in.Weather {
		sc, wx, _ = s.dayContext(ctx, s.clients(ctx, uid), date, in.Occasion, in.Style)
	} else {
		sc = s.baseContext(ctx, uid, in.Style)
		sc.Occasion = firstNonEmpty(in.Occasion, "everyday")
	}
	pieces, _ := s.pieces(ctx, uid, date)
	var ps []style.Piece
	for _, p := range pieces {
		if hasInt(in.ItemIDs, p.ID) {
			ps = append(ps, p)
		}
	}
	res := style.Rate(ps, sc)
	if len(ps) == 0 {
		res = style.Score{Items: []int64{}, Reasons: []string{}, Warnings: []string{"pick a few pieces"}, Colors: []string{}}
	}
	writeJSON(w, http.StatusOK, map[string]any{"rating": res, "weather": wx})
}

// handleStyles is what the engine knows: styles, occasions, kinds, colours.
func (s *Server) handleStyles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"styles": style.Styles, "occasions": style.Occasions, "kinds": style.Kinds, "categories": style.Categories,
		"families": style.Families, "patterns": style.Patterns, "seasons": style.Seasons})
}

// ---- pictures of outfits (for Pocket Chat and anything without a session) ----

// shareKey is the box's secret for picture links (made once, kept in the cache table).
func (s *Server) shareKey(ctx context.Context) []byte {
	if v, _, ok := s.store.CacheGet(ctx, "share-key"); ok && len(v) == 64 {
		b, _ := hex.DecodeString(v)
		return b
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	_ = s.store.CachePut(ctx, "share-key", hex.EncodeToString(b))
	return b
}

func (s *Server) shareSig(ctx context.Context, uid int64, ids string) string {
	m := hmac.New(sha256.New, s.shareKey(ctx))
	m.Write([]byte(itoa(uid) + ":" + ids))
	return hex.EncodeToString(m.Sum(nil))[:24]
}

// shareURL is a picture of pieces together that opens without signing in
// (only someone with the link can see it; it shows nothing but the pieces).
func (s *Server) shareURL(uid int64, ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, itoa(id))
	}
	j := strings.Join(parts, "-")
	return s.link("/s/" + itoa(uid) + "/" + j + "/" + s.shareSig(context.Background(), uid, j) + ".jpg")
}

func (s *Server) handleSharedCollage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid, _ := strconv.ParseInt(chiParam(r, "uid"), 10, 64)
	j := chiParam(r, "ids")
	sig := strings.TrimSuffix(chiParam(r, "sig"), ".jpg")
	if uid <= 0 || !hmac.Equal([]byte(sig), []byte(s.shareSig(ctx, uid, j))) {
		http.NotFound(w, r)
		return
	}
	var ids []int64
	for _, p := range strings.Split(j, "-") {
		if id, err := strconv.ParseInt(p, 10, 64); err == nil && len(ids) < 12 {
			ids = append(ids, id)
		}
	}
	s.writeCollage(w, r, uid, ids)
}

func (s *Server) handleCollage(w http.ResponseWriter, r *http.Request) {
	var ids []int64
	for _, p := range strings.Split(r.URL.Query().Get("ids"), ",") {
		if id, err := strconv.ParseInt(p, 10, 64); err == nil && len(ids) < 12 {
			ids = append(ids, id)
		}
	}
	s.writeCollage(w, r, actorID(r.Context()), ids)
}

func (s *Server) writeCollage(w http.ResponseWriter, r *http.Request, uid int64, ids []int64) {
	ctx := r.Context()
	items, err := s.store.ItemsByIDs(ctx, uid, ids)
	if err != nil || len(items) == 0 {
		http.NotFound(w, r)
		return
	}
	order := map[string]int{"outerwear": 0, "knit": 1, "top": 2, "dress": 2, "bottom": 3, "shoes": 4, "bag": 5, "accessory": 6}
	var list []*store.Item
	for _, id := range ids {
		if it := items[id]; it != nil {
			list = append(list, it)
		}
	}
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && order[list[j].Category] < order[list[j-1].Category]; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
	cols := 2
	if len(list) == 1 {
		cols = 1
	} else if len(list) > 4 {
		cols = 3
	}
	rowsN := (len(list) + cols - 1) / cols
	const tile, gap = 320, 12
	W, H := cols*tile+(cols+1)*gap, rowsN*tile+(rowsN+1)*gap
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0xf4, 0xf2, 0xee, 0xff}}, image.Point{}, draw.Src)
	for i, it := range list {
		x, y := gap+(i%cols)*(tile+gap), gap+(i/cols)*(tile+gap)
		cell := image.Rect(x, y, x+tile, y+tile)
		draw.Draw(img, cell, &image.Uniform{color.White}, image.Point{}, draw.Src)
		var src image.Image
		if it.PhotoID != nil {
			if p, err := s.store.GetPhoto(ctx, uid, *it.PhotoID, true); err == nil {
				src, _, _ = image.Decode(bytes.NewReader(p.Data))
			}
		}
		if src != nil {
			fit(img, cell.Inset(6), src)
			continue
		}
		hexs := "#888888"
		if len(it.Colors) > 0 {
			hexs = it.Colors[0].Hex
		}
		rr, gg, bb, _ := style.ParseHex(hexs)
		swatch := cell.Inset(tile / 5)
		draw.Draw(img, swatch, &image.Uniform{color.RGBA{uint8(rr * 255), uint8(gg * 255), uint8(bb * 255), 0xff}}, image.Point{}, draw.Src)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 84}); err != nil {
		s.fail(w, err, "picture")
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=600")
	_, _ = w.Write(buf.Bytes())
}

// fit draws src inside r, whole and centred, averaging the pixels it shrinks.
func fit(dst *image.RGBA, r image.Rectangle, src image.Image) {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw == 0 || sh == 0 {
		return
	}
	scale := float64(r.Dx()) / float64(sw)
	if v := float64(r.Dy()) / float64(sh); v < scale {
		scale = v
	}
	dw, dh := int(float64(sw)*scale), int(float64(sh)*scale)
	ox, oy := r.Min.X+(r.Dx()-dw)/2, r.Min.Y+(r.Dy()-dh)/2
	step := 1 / scale
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			x0, y0 := sb.Min.X+int(float64(x)*step), sb.Min.Y+int(float64(y)*step)
			x1, y1 := sb.Min.X+int(float64(x+1)*step), sb.Min.Y+int(float64(y+1)*step)
			if x1 <= x0 {
				x1 = x0 + 1
			}
			if y1 <= y0 {
				y1 = y0 + 1
			}
			var rs, gs, bs, n uint32
			for yy := y0; yy < y1 && yy < sb.Max.Y; yy += 1 + (y1-y0)/4 {
				for xx := x0; xx < x1 && xx < sb.Max.X; xx += 1 + (x1-x0)/4 {
					cr, cg, cb, _ := src.At(xx, yy).RGBA()
					rs, gs, bs, n = rs+cr, gs+cg, bs+cb, n+1
				}
			}
			if n > 0 {
				dst.SetRGBA(ox+x, oy+y, color.RGBA{uint8(rs / n >> 8), uint8(gs / n >> 8), uint8(bs / n >> 8), 0xff})
			}
		}
	}
}
