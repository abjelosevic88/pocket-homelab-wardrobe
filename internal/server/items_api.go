package server

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg" // decode uploads
	_ "image/png"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/style"
)

// ---- pieces ----

type itemInput struct {
	Name       *string        `json:"name"`
	Category   *string        `json:"category"`
	Kind       *string        `json:"kind"`
	Colors     *[]store.Color `json:"colors"`
	Pattern    *string        `json:"pattern"`
	Material   *string        `json:"material"`
	Fit        *string        `json:"fit"`
	Seasons    *[]string      `json:"seasons"`
	Warmth     *int           `json:"warmth"`
	Formality  *int           `json:"formality"`
	Styles     *[]string      `json:"styles"`
	Rain       *bool          `json:"rain"`
	Brand      *string        `json:"brand"`
	Size       *string        `json:"size"`
	Price      *int64         `json:"price"`
	NoPrice    bool           `json:"no_price"`
	Currency   *string        `json:"currency"`
	BoughtOn   *string        `json:"bought_on"`
	Shop       *string        `json:"shop"`
	Care       *string        `json:"care"`
	Notes      *string        `json:"notes"`
	Status     *string        `json:"status"`
	StatusNote *string        `json:"status_note"`
	Favourite  *bool          `json:"favourite"`
	PhotoID    *int64         `json:"photo_id"`
	SourceRef  *string        `json:"source_ref"`
	Source     *string        `json:"source"`
}

// apply checks and copies what was sent; "" = fine, else what is wrong.
func (in itemInput) apply(it *store.Item) string {
	t := func(p *string, n int) string { return clip(strings.TrimSpace(*p), n) }
	if in.Kind != nil {
		k := strings.TrimSpace(*in.Kind)
		if k != "" {
			kd, ok := style.KindByID(k)
			if !ok {
				return "unknown kind of piece"
			}
			if in.Category == nil {
				it.Category = kd.Category
			}
			if in.Warmth == nil && it.ID == 0 {
				it.Warmth = kd.Warmth
			}
			if in.Formality == nil && it.ID == 0 {
				it.Formality = kd.Formality
			}
			if in.Rain == nil && it.ID == 0 {
				it.Rain = kd.Rain
			}
		}
		it.Kind = k
	}
	if in.Category != nil {
		ok := false
		for _, c := range style.Categories {
			if c.ID == *in.Category {
				ok = true
			}
		}
		if !ok {
			return "unknown category"
		}
		it.Category = *in.Category
	}
	if in.Name != nil {
		it.Name = t(in.Name, 120)
	}
	if in.Colors != nil {
		cs := []store.Color{}
		for _, c := range *in.Colors {
			hex := strings.ToLower(strings.TrimSpace(c.Hex))
			if !style.ValidHex(hex) {
				if f, ok := style.FamilyByName(c.Name); ok {
					hex = f.Hex
				} else {
					continue
				}
			}
			name := strings.ToLower(strings.TrimSpace(c.Name))
			if _, ok := style.FamilyByName(name); !ok {
				name = style.FamilyOf(hex)
			}
			if len(cs) < 4 {
				cs = append(cs, store.Color{Hex: hex, Name: name})
			}
		}
		it.Colors = cs
	}
	if in.Pattern != nil {
		if !oneOf(*in.Pattern, style.Patterns) {
			return "unknown pattern"
		}
		it.Pattern = *in.Pattern
	}
	if in.Material != nil {
		it.Material = t(in.Material, 80)
	}
	if in.Fit != nil {
		if !oneOf(*in.Fit, store.Fits) {
			return "the fit is slim, regular, relaxed or oversized"
		}
		it.Fit = *in.Fit
	}
	if in.Seasons != nil {
		ss := []string{}
		for _, v := range *in.Seasons {
			if oneOf(v, style.Seasons) && !oneOf(v, ss) {
				ss = append(ss, v)
			}
		}
		if len(ss) == 4 {
			ss = []string{} // all year
		}
		it.Seasons = ss
	}
	if in.Warmth != nil {
		if *in.Warmth < 0 || *in.Warmth > 5 {
			return "warmth is 0 to 5"
		}
		it.Warmth = *in.Warmth
	}
	if in.Formality != nil {
		if *in.Formality < 1 || *in.Formality > 5 {
			return "how smart it is goes from 1 to 5"
		}
		it.Formality = *in.Formality
	}
	if in.Styles != nil {
		ss := []string{}
		for _, v := range *in.Styles {
			if _, ok := style.StyleByID(v); ok && !oneOf(v, ss) {
				ss = append(ss, v)
			}
		}
		it.Styles = ss
	}
	if in.Rain != nil {
		it.Rain = *in.Rain
	}
	if in.Brand != nil {
		it.Brand = t(in.Brand, 80)
	}
	if in.Size != nil {
		it.Size = t(in.Size, 20)
	}
	if in.NoPrice {
		it.Price = nil
	} else if in.Price != nil {
		if *in.Price < 0 {
			return "the price cannot be negative"
		}
		v := *in.Price
		it.Price = &v
	}
	if in.Currency != nil {
		it.Currency = cleanCurrency(*in.Currency)
	}
	if in.BoughtOn != nil {
		if !validDate(*in.BoughtOn) {
			return "the date bought is YYYY-MM-DD"
		}
		it.BoughtOn = *in.BoughtOn
	}
	if in.Shop != nil {
		it.Shop = t(in.Shop, 80)
	}
	if in.Care != nil {
		it.Care = t(in.Care, 300)
	}
	if in.Notes != nil {
		it.Notes = t(in.Notes, 2000)
	}
	if in.Status != nil {
		if !oneOf(*in.Status, store.ItemStates) {
			return "unknown state"
		}
		if *in.Status != it.Status {
			it.StatusAt = today()
		}
		it.Status = *in.Status
	}
	if in.StatusNote != nil {
		it.StatusNote = t(in.StatusNote, 200)
	}
	if in.Favourite != nil {
		it.Favourite = *in.Favourite
	}
	if in.SourceRef != nil {
		it.SourceRef = t(in.SourceRef, 120)
	}
	if it.Name == "" {
		if k, ok := style.KindByID(it.Kind); ok {
			it.Name = k.Label
			if len(it.Colors) > 0 {
				it.Name = strings.ToUpper(it.Colors[0].Name[:1]) + it.Colors[0].Name[1:] + " " + strings.ToLower(k.Label)
			}
		}
	}
	if it.Name == "" {
		return "give it a name or say what it is"
	}
	return ""
}

func photoURL(id *int64) (string, string) {
	if id == nil {
		return "", ""
	}
	u := "/api/v1/photos/" + itoa(*id)
	return u, u + "?thumb=1"
}

func decorateItem(it *store.Item) *store.Item {
	it.PhotoURL, it.ThumbURL = photoURL(it.PhotoID)
	return it
}

func (s *Server) handleListItems(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	q := r.URL.Query()
	list, err := s.store.ListItems(ctx, uid, store.ItemFilter{Category: q.Get("category"), Status: q.Get("status"), Q: q.Get("q"), Gone: queryBool(r, "gone")})
	if err != nil {
		s.fail(w, err, "items")
		return
	}
	counts := map[string]int{}
	states := map[string]int{}
	all, _ := s.store.ListItems(ctx, uid, store.ItemFilter{})
	for _, it := range all {
		counts[it.Category]++
		states[it.Status]++
	}
	for _, it := range list {
		decorateItem(it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": list, "counts": counts, "states": states, "total": len(all)})
}

func (s *Server) ownItem(w http.ResponseWriter, r *http.Request, write bool) *store.Item {
	ctx := r.Context()
	it, err := s.store.GetItem(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "item")
		return nil
	}
	if app := linkApp(ctx); write && app != "" && it.Source != app {
		writeErr(w, http.StatusNotFound, "not found")
		return nil
	}
	return it
}

func (s *Server) handleGetItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	it := s.ownItem(w, r, false)
	if it == nil {
		return
	}
	decorateItem(it)
	days, _ := s.store.ItemWears(ctx, uid, it.ID, 60)
	outfits, _ := s.store.ListOutfits(ctx, uid)
	in := []*store.Outfit{}
	for _, o := range outfits {
		if hasInt(o.ItemIDs, it.ID) {
			s.decorateOutfit(o)
			in = append(in, o)
		}
	}
	pieces, items := s.pieces(ctx, uid, today())
	var me style.Piece
	for _, p := range pieces {
		if p.ID == it.ID {
			me = p
		}
	}
	similar := []*store.Item{}
	for _, p := range style.Similar(pieces, me) {
		if x := items[p.ID]; x != nil {
			similar = append(similar, decorateItem(x))
		}
	}
	// what it goes with best, per kind of piece
	c := s.baseContext(ctx, uid, "")
	c.Season = ""
	type pair struct {
		it    *store.Item
		score float64
	}
	var pairs []pair
	for _, p := range pieces {
		if p.ID == it.ID || p.Category == it.Category || p.Category == "accessory" || p.Category == "bag" || p.Category == "other" {
			continue
		}
		if (it.Category == "dress" && (p.Category == "top" || p.Category == "bottom")) || (p.Category == "dress" && (it.Category == "top" || it.Category == "bottom")) {
			continue
		}
		sc := style.Rate([]style.Piece{me, p}, c)
		pairs = append(pairs, pair{items[p.ID], sc.Score - float64(len(sc.Warnings))*4})
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].score > pairs[j].score })
	goes := []*store.Item{}
	for i := 0; i < len(pairs) && len(goes) < 8; i++ {
		goes = append(goes, decorateItem(pairs[i].it))
	}
	out := map[string]any{"item": it, "worn": days, "outfits": in, "similar": similar, "goes_with": goes}
	if it.Wears == 0 && it.BoughtOn != "" && daysBetween(it.BoughtOn, today()) > 180 {
		out["nudge"] = "Not worn since it came in. Wear it this week, or let it go to someone who will."
	} else if it.LastWorn != "" && daysBetween(it.LastWorn, today()) > 365 {
		out["nudge"] = "Not worn for over a year."
	}
	writeJSON(w, http.StatusOK, out)
}

func hasInt(list []int64, v int64) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (s *Server) handleCreateItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in itemInput
	if _, err := decodeKeys(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	st := s.settingsOf(ctx, uid)
	it := &store.Item{UserID: uid, Category: "top", Pattern: "plain", Warmth: 2, Formality: 2, Status: "ready", Currency: st.Currency, Colors: []store.Color{}, Seasons: []string{}, Styles: []string{}, Source: linkApp(ctx)}
	if it.Source == "" && in.Source != nil && oneOf(*in.Source, []string{"", "ai", "money", "parcels", "wishlist", "idea"}) {
		it.Source = *in.Source
	}
	if msg := in.apply(it); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if id, ok := s.photoMine(ctx, uid, in.PhotoID); !ok {
		writeErr(w, http.StatusBadRequest, "unknown photo")
		return
	} else {
		it.PhotoID = id
	}
	if err := s.store.CreateItem(ctx, it); err != nil {
		s.fail(w, err, "create item")
		return
	}
	it, _ = s.store.GetItem(ctx, uid, it.ID)
	s.publish(ctx, uid, "wardrobe.item_added", "item:"+itoa(it.ID), map[string]any{"id": it.ID, "name": it.Name, "category": it.Category, "kind": it.Kind, "url": s.link("/closet/" + itoa(it.ID))})
	writeJSON(w, http.StatusCreated, decorateItem(it))
}

// photoMine checks that a photo sent with something is the person's own
// (nil or 0 = no photo).
func (s *Server) photoMine(ctx context.Context, uid int64, in *int64) (*int64, bool) {
	if in == nil || *in == 0 {
		return nil, true
	}
	if _, err := s.store.GetPhoto(ctx, uid, *in, true); err != nil {
		return nil, false
	}
	v := *in
	return &v, true
}

func (s *Server) handleUpdateItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	it := s.ownItem(w, r, true)
	if it == nil {
		return
	}
	var in itemInput
	if _, err := decodeKeys(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if msg := in.apply(it); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	old := it.PhotoID
	if in.PhotoID != nil {
		id, ok := s.photoMine(ctx, uid, in.PhotoID)
		if !ok {
			writeErr(w, http.StatusBadRequest, "unknown photo")
			return
		}
		it.PhotoID = id
	}
	if err := s.store.UpdateItem(ctx, it); err != nil {
		s.fail(w, err, "update item")
		return
	}
	if old != nil && (it.PhotoID == nil || *old != *it.PhotoID) {
		_ = s.store.DeletePhotoIfUnused(ctx, uid, *old)
	}
	it, _ = s.store.GetItem(ctx, uid, it.ID)
	writeJSON(w, http.StatusOK, decorateItem(it))
}

func (s *Server) handleDeleteItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	it := s.ownItem(w, r, true)
	if it == nil {
		return
	}
	if err := s.store.DeleteItem(ctx, actorID(ctx), it.ID); err != nil {
		s.fail(w, err, "delete item")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// handleItemStatus moves pieces between the closet, the laundry, the
// tailor and so on; "all laundry clean" is status ready with from=laundry.
func (s *Server) handleItemStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		IDs    []int64 `json:"ids"`
		From   string  `json:"from"`
		Status string  `json:"status"`
		Note   string  `json:"note"`
	}
	if err := decode(r, &in); err != nil || !oneOf(in.Status, store.ItemStates) {
		writeErr(w, http.StatusBadRequest, "say which pieces and where they are")
		return
	}
	ids := in.IDs
	if in.From != "" {
		list, err := s.store.ListItems(ctx, uid, store.ItemFilter{Status: in.From})
		if err != nil {
			s.fail(w, err, "items")
			return
		}
		ids = nil
		for _, it := range list {
			ids = append(ids, it.ID)
		}
	}
	n, err := s.store.SetItemStatus(ctx, uid, ids, in.Status, clip(strings.TrimSpace(in.Note), 200))
	if err != nil {
		s.fail(w, err, "status")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"changed": n})
}

// handleItemGone marks a piece as given away, sold, thrown out or lost
// (it leaves the closet but its history stays); reason "" brings it back.
func (s *Server) handleItemGone(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	it := s.ownItem(w, r, true)
	if it == nil {
		return
	}
	var in struct {
		Reason  string `json:"reason"`
		Date    string `json:"date"`
		SoldFor *int64 `json:"sold_for"`
	}
	if err := decode(r, &in); err != nil || !oneOf(in.Reason, store.GoneReasons) || !validDate(in.Date) {
		writeErr(w, http.StatusBadRequest, "the reason is donated, sold, thrown or lost")
		return
	}
	it.Gone, it.GoneOn, it.SoldFor = in.Reason, firstNonEmpty(in.Date, today()), in.SoldFor
	if in.Reason == "" {
		it.GoneOn, it.SoldFor = "", nil
	}
	if err := s.store.UpdateItem(ctx, it); err != nil {
		s.fail(w, err, "update item")
		return
	}
	writeJSON(w, http.StatusOK, decorateItem(it))
}

// ---- photos ----

// maxPhoto is the largest picture taken (the browser shrinks them first).
const maxPhoto = 8 << 20

func (s *Server) handleUploadPhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	r.Body = http.MaxBytesReader(w, r.Body, 2*maxPhoto+(1<<20))
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "the picture is too big (8 MB at most)")
		return
	}
	read := func(field string) ([]byte, string, error) {
		f, h, err := r.FormFile(field)
		if err != nil {
			return nil, "", err
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, maxPhoto+1))
		if err != nil {
			return nil, "", err
		}
		if len(b) > maxPhoto {
			return nil, "", fmt.Errorf("too big")
		}
		return b, h.Header.Get("Content-Type"), nil
	}
	data, mime, err := read("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "send the picture as “file”")
		return
	}
	mime = http.DetectContentType(data)
	if mime != "image/jpeg" && mime != "image/png" && mime != "image/webp" {
		writeErr(w, http.StatusBadRequest, "send a JPEG, PNG or WebP picture")
		return
	}
	p := &store.Photo{UserID: uid, Kind: firstNonEmpty(r.FormValue("kind"), "item"), Mime: mime, Data: data}
	if !oneOf(p.Kind, []string{"item", "look", "check"}) {
		p.Kind = "item"
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		p.Width, p.Height = cfg.Width, cfg.Height
	} else {
		p.Width, _ = strconv.Atoi(r.FormValue("width"))
		p.Height, _ = strconv.Atoi(r.FormValue("height"))
	}
	if t, tm, err := read("thumb"); err == nil && len(t) > 0 {
		if http.DetectContentType(t) == "image/jpeg" || tm == "image/jpeg" {
			p.Thumb = t
		}
	}
	if err := s.store.CreatePhoto(ctx, p); err != nil {
		s.fail(w, err, "photo")
		return
	}
	u, t := photoURL(&p.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"id": p.ID, "url": u, "thumb_url": t, "width": p.Width, "height": p.Height})
}

func (s *Server) handlePhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, err := s.store.GetPhoto(ctx, actorID(ctx), idParam(r, "id"), queryBool(r, "thumb"))
	if err != nil {
		s.fail(w, err, "photo")
		return
	}
	w.Header().Set("Content-Type", p.Mime)
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(p.Data))
}

// ---- the other apps for one piece ----

// handleItemTask puts "take it to the tailor", "dry clean", "return it" on Pocket Tasks.
func (s *Server) handleItemTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	it := s.ownItem(w, r, false)
	if it == nil {
		return
	}
	var in struct {
		What string `json:"what"` // tailor | dry_clean | repair | return | sell
		Due  string `json:"due"`
		Note string `json:"note"`
	}
	if err := decode(r, &in); err != nil || !validDate(in.Due) {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	title := map[string]string{"tailor": "Take to the tailor: ", "dry_clean": "Dry clean: ", "repair": "Repair: ", "return": "Return: ", "sell": "Sell or give away: "}[in.What]
	if title == "" {
		writeErr(w, http.StatusBadRequest, "say what to do with it")
		return
	}
	c := s.clients(ctx, uid)
	sib, err := c.conn(ctx, appTasks, scopeTasksWrite)
	if err != nil {
		s.userFail(w, err, "tasks")
		return
	}
	body := map[string]any{"title": clip(title+it.Name, 200), "notes": strings.TrimSpace(clip(in.Note, 1000) + "\n\n" + s.link("/closet/"+itoa(it.ID)))}
	if in.Due != "" {
		body["due"] = in.Due
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/tasks", body)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "Pocket Tasks: "+err.Error())
		return
	}
	if in.What == "tailor" || in.What == "repair" || in.What == "dry_clean" {
		st := "repair"
		_, _ = s.store.SetItemStatus(ctx, uid, []int64{it.ID}, st, map[string]string{"tailor": "at the tailor", "repair": "to repair", "dry_clean": "at the dry cleaner"}[in.What])
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": linkTo(sib, "/tasks/"+idOf(res)), "text": "On your task list"})
}
