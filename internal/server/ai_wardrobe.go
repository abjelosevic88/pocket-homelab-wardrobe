package server

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/style"
)

// ---- Pocket AI (or the person's own OpenAI key) ----
//
// Only when the person presses a button: what a photo shows (one piece or
// several), outfits for a wish ("dinner, relaxed but smart"), what goes
// with a piece, a look rated from a mirror photo, the person's style and
// colours, what to buy, "should I buy this?" and a trip's packing. The
// answers are checked against the person's own pieces before they show.

func kindIDs() string {
	var b []string
	for _, k := range style.Kinds {
		b = append(b, k.ID)
	}
	return strings.Join(b, ", ")
}

func familyNames() string {
	var b []string
	for _, f := range style.Families {
		b = append(b, f.Name)
	}
	return strings.Join(b, ", ")
}

func styleIDs() string {
	var b []string
	for _, s := range style.Styles {
		b = append(b, s.ID+" ("+s.Name+")")
	}
	return strings.Join(b, ", ")
}

// closetText is the wardrobe as lines the AI can pick ids from.
func closetText(items []*store.Item, onlyReady bool) string {
	var b strings.Builder
	for _, it := range items {
		if onlyReady && it.Status != "ready" {
			continue
		}
		var cs []string
		for _, c := range it.Colors {
			cs = append(cs, c.Name)
		}
		fmt.Fprintf(&b, "%d | %s | %s/%s | %s | %s | warmth %d | smart %d", it.ID, it.Name, it.Category, it.Kind, strings.Join(cs, "+"), it.Pattern, it.Warmth, it.Formality)
		if len(it.Styles) > 0 {
			b.WriteString(" | " + strings.Join(it.Styles, ","))
		}
		if it.Status != "ready" {
			b.WriteString(" | " + it.Status)
		}
		if it.Wears > 0 {
			fmt.Fprintf(&b, " | worn %d×", it.Wears)
		}
		if it.Favourite {
			b.WriteString(" | favourite")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (s *Server) personText(st store.Settings) string {
	var b strings.Builder
	var names []string
	for _, id := range st.Styles {
		if sv, ok := style.StyleByID(id); ok {
			names = append(names, sv.Name)
		}
	}
	if len(names) > 0 {
		b.WriteString("Their styles: " + strings.Join(names, ", ") + ".\n")
	}
	if st.Presentation != "" && st.Presentation != "any" {
		b.WriteString("They wear " + st.Presentation + ".\n")
	}
	if len(st.LoveColors) > 0 {
		b.WriteString("Colours they like: " + strings.Join(st.LoveColors, ", ") + ".\n")
	}
	if len(st.AvoidColors) > 0 {
		b.WriteString("Colours they avoid: " + strings.Join(st.AvoidColors, ", ") + ".\n")
	}
	if st.ColorSeason != "" {
		b.WriteString("Their colour season: " + st.ColorSeason + ".\n")
	}
	if st.FitNotes != "" {
		b.WriteString("About fit: " + clip(st.FitNotes, 400) + "\n")
	}
	return b.String()
}

const aiVoice = "You are a friendly, practical personal stylist inside Pocket Wardrobe. Plain words, no fashion jargon, short sentences. Be kind and honest."

// aiStart checks the switch and takes one of the AI slots.
func (s *Server) aiStart(w http.ResponseWriter, r *http.Request) (store.Settings, func(), bool) {
	st := s.settingsOf(r.Context(), actorID(r.Context()))
	if !st.AIHelp {
		writeErr(w, http.StatusBadRequest, "AI help is switched off (Settings → AI)")
		return st, nil, false
	}
	select {
	case s.aiSem <- struct{}{}:
		return st, func() { <-s.aiSem }, true
	default:
		writeErr(w, http.StatusTooManyRequests, "the AI is busy; try again in a moment")
		return st, nil, false
	}
}

// ---- what a photo shows ----

type aiPiece struct {
	Name      string        `json:"name"`
	Category  string        `json:"category"`
	Kind      string        `json:"kind"`
	Colors    []store.Color `json:"colors"`
	Pattern   string        `json:"pattern"`
	Material  string        `json:"material"`
	Fit       string        `json:"fit"`
	Seasons   []string      `json:"seasons"`
	Warmth    int           `json:"warmth"`
	Formality int           `json:"formality"`
	Styles    []string      `json:"styles"`
	Rain      bool          `json:"rain"`
	Brand     string        `json:"brand"`
	Care      string        `json:"care"`
	Notes     string        `json:"notes"`
}

// clean keeps what the AI said within what the app knows.
func (p *aiPiece) clean() {
	if k, ok := style.KindByID(p.Kind); ok {
		p.Category = k.Category
		if p.Warmth < 0 || p.Warmth > 5 {
			p.Warmth = k.Warmth
		}
		if p.Formality < 1 || p.Formality > 5 {
			p.Formality = k.Formality
		}
	} else {
		p.Kind = "other"
		p.Category = "other"
	}
	cs := []store.Color{}
	for _, c := range p.Colors {
		hex := strings.ToLower(strings.TrimSpace(c.Hex))
		if !style.ValidHex(hex) {
			hex = style.HexOf(c.Name)
		}
		name := strings.ToLower(c.Name)
		if _, ok := style.FamilyByName(name); !ok {
			name = style.FamilyOf(hex)
		}
		if len(cs) < 3 {
			cs = append(cs, store.Color{Hex: hex, Name: name})
		}
	}
	p.Colors = cs
	if !oneOf(p.Pattern, style.Patterns) {
		p.Pattern = "plain"
	}
	if !oneOf(p.Fit, store.Fits) {
		p.Fit = ""
	}
	var ss []string
	for _, v := range p.Seasons {
		if oneOf(v, style.Seasons) {
			ss = append(ss, v)
		}
	}
	p.Seasons = ss
	var sts []string
	for _, v := range p.Styles {
		if _, ok := style.StyleByID(v); ok {
			sts = append(sts, v)
		}
	}
	p.Styles = sts
	p.Name, p.Material, p.Brand, p.Care, p.Notes = clip(p.Name, 120), clip(p.Material, 80), clip(p.Brand, 80), clip(p.Care, 300), clip(p.Notes, 300)
}

var pieceSchema = `{"name": "short plain name, e.g. Navy wool jumper", "kind": one of [` + kindIDs() + `], "colors": [{"hex": "#rrggbb", "name": one of [` + familyNames() + `]}] (main colour first, up to 3),
"pattern": one of [plain, stripes, checks, dots, floral, print, other], "material": "cotton, wool, denim, leather… or empty", "fit": one of ["", slim, regular, relaxed, oversized],
"seasons": subset of [spring, summer, autumn, winter], "warmth": 0-5 (1 very light … 5 very warm), "formality": 1-5 (1 sporty, 2 casual, 3 smart casual, 4 business, 5 black tie),
"styles": subset of [` + styleIDs() + `] (ids only), "rain": true if it keeps rain out, "brand": "if a label is readable, else empty", "care": "a care tip if obvious, else empty", "notes": "anything worth knowing, else empty"}`

func (s *Server) handleAITag(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		PhotoID int64  `json:"photo_id"`
		Many    bool   `json:"many"`
		Hint    string `json:"hint"`
	}
	if err := decode(r, &in); err != nil || in.PhotoID == 0 {
		writeErr(w, http.StatusBadRequest, "send a photo first")
		return
	}
	st, done, ok := s.aiStart(w, r)
	if !ok {
		return
	}
	defer done()
	imgs, err := s.photoForAI(ctx, uid, in.PhotoID)
	if err != nil {
		s.fail(w, err, "photo")
		return
	}
	hint := ""
	if h := strings.TrimSpace(in.Hint); h != "" {
		hint = "\nThe person says: " + clip(h, 300)
	}
	if in.Many {
		var out struct {
			Items []aiPiece `json:"items"`
		}
		err = s.aiAsk(ctx, uid, st, aiVoice, "List every piece of clothing, shoes, bag or accessory you can see in this photo (laid out, on a rail or worn). For each give: "+pieceSchema+`
Answer {"items": [ … ]} with at most 12 items.`+hint, imgs, &out)
		if err != nil {
			s.userFail(w, err, "AI")
			return
		}
		for i := range out.Items {
			out.Items[i].clean()
		}
		writeJSON(w, http.StatusOK, out)
		return
	}
	var p aiPiece
	err = s.aiAsk(ctx, uid, st, aiVoice, "Describe the one piece of clothing (or shoes, bag, accessory) in this photo so it can go into a wardrobe app. If there are several, describe the main one. Answer one JSON object: "+pieceSchema+hint, imgs, &p)
	if err != nil {
		s.userFail(w, err, "AI")
		return
	}
	p.clean()
	writeJSON(w, http.StatusOK, map[string]any{"item": p})
}

// ---- outfits the AI puts together ----

type aiOutfit struct {
	Name    string  `json:"name"`
	ItemIDs []int64 `json:"item_ids"`
	Why     string  `json:"why"`
}

func (s *Server) cleanAIOutfits(ctx context.Context, uid int64, list []aiOutfit, c style.Context, pieces []style.Piece) []map[string]any {
	byID := map[int64]style.Piece{}
	for _, p := range pieces {
		byID[p.ID] = p
	}
	out := []map[string]any{}
	for _, o := range list {
		var ps []style.Piece
		ids := []int64{}
		for _, id := range o.ItemIDs {
			if p, ok := byID[id]; ok && !hasInt(ids, id) {
				ps = append(ps, p)
				ids = append(ids, id)
			}
		}
		if len(ids) < 2 {
			continue
		}
		sc := style.Rate(ps, c)
		out = append(out, map[string]any{"name": clip(o.Name, 80), "item_ids": ids, "why": clip(o.Why, 600), "rating": sc})
		if len(out) >= 4 {
			break
		}
	}
	return out
}

func (s *Server) handleAIStyleMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		Date     string `json:"date"`
		Occasion string `json:"occasion"`
		Style    string `json:"style"`
		Wish     string `json:"wish"`
		ItemID   int64  `json:"item_id"` // "what goes with this"
	}
	if err := decode(r, &in); err != nil || !validDate(in.Date) {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	st, done, ok := s.aiStart(w, r)
	if !ok {
		return
	}
	defer done()
	date := firstNonEmpty(in.Date, today())
	c := s.clients(ctx, uid)
	sc, wx, _ := s.dayContext(ctx, c, date, firstNonEmpty(in.Occasion, "everyday"), in.Style)
	pieces, byID := s.pieces(ctx, uid, date)
	var items []*store.Item
	for _, it := range byID {
		items = append(items, it)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if len(items) < 3 {
		writeErr(w, http.StatusBadRequest, "add a few pieces first")
		return
	}
	var b strings.Builder
	b.WriteString(s.personText(st))
	occ := style.OccasionByID(sc.Occasion)
	fmt.Fprintf(&b, "Day: %s. Occasion: %s.\n", niceDate(date), occ.Label)
	if sv, ok := style.StyleByID(sc.Style); ok {
		fmt.Fprintf(&b, "Aim for: %s — %s\n", sv.Name, sv.Description)
	}
	if wx != nil {
		fmt.Fprintf(&b, "Weather: %s.\n", wx.short())
	}
	if w := strings.TrimSpace(in.Wish); w != "" {
		b.WriteString("What they want: " + clip(w, 500) + "\n")
	}
	if in.ItemID > 0 {
		if it := byID[in.ItemID]; it != nil {
			fmt.Fprintf(&b, "Every outfit must include piece %d (%s).\n", it.ID, it.Name)
		}
	}
	b.WriteString("\nTheir wardrobe (id | name | category/kind | colours | pattern | warmth | smart | styles | state | wears):\n")
	b.WriteString(closetText(items, true))
	b.WriteString(`
Put together 3 different complete outfits from these pieces only (by id): a top and a bottom, or a dress; shoes; a jacket or coat when the weather needs one; at most one bag and two accessories.
Only pieces that are ready (not in the laundry, at the tailor or lent). Prefer pieces worn less often when they work as well.
Answer {"outfits": [{"name": "short name", "item_ids": [ids], "why": "one or two sentences on why it works for this day"}], "tip": "one practical styling tip for today"}`)
	var out struct {
		Outfits []aiOutfit `json:"outfits"`
		Tip     string     `json:"tip"`
	}
	if err := s.aiAsk(ctx, uid, st, aiVoice, b.String(), nil, &out); err != nil {
		s.userFail(w, err, "AI")
		return
	}
	res := s.cleanAIOutfits(ctx, uid, out.Outfits, sc, pieces)
	used := map[int64]*store.Item{}
	for _, o := range res {
		for _, id := range o["item_ids"].([]int64) {
			used[id] = decorateItem(byID[id])
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"outfits": res, "tip": clip(out.Tip, 400), "items": used, "weather": wx})
}

// ---- a look, from a mirror photo ----

func (s *Server) handleAIRateLook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		PhotoID  int64  `json:"photo_id"`
		Occasion string `json:"occasion"`
		Question string `json:"question"`
	}
	if err := decode(r, &in); err != nil || in.PhotoID == 0 {
		writeErr(w, http.StatusBadRequest, "send a photo of the look first")
		return
	}
	st, done, ok := s.aiStart(w, r)
	if !ok {
		return
	}
	defer done()
	imgs, err := s.photoForAI(ctx, uid, in.PhotoID)
	if err != nil {
		s.fail(w, err, "photo")
		return
	}
	items, _ := s.store.ListItems(ctx, uid, store.ItemFilter{Status: "ready"})
	occ := style.OccasionByID(firstNonEmpty(in.Occasion, "everyday"))
	prompt := s.personText(st) + "Occasion: " + occ.Label + ".\n"
	if q := strings.TrimSpace(in.Question); q != "" {
		prompt += "They ask: " + clip(q, 400) + "\n"
	}
	prompt += "\nTheir wardrobe (id | name | …), for swaps:\n" + closetText(items, true) + `
Look at the outfit they are wearing in the photo. Judge the outfit, never the body. Be encouraging and specific.
Answer {"score": 1-10, "headline": "one short sentence", "good": ["what works", …up to 3], "change": ["what to change", …up to 3],
"swaps": [{"instead_of": "the piece in the photo", "item_id": id from their wardrobe or 0, "why": "short"}] (up to 2), "occasion_fit": "is it right for the occasion, one sentence"}`
	var out struct {
		Score       int      `json:"score"`
		Headline    string   `json:"headline"`
		Good        []string `json:"good"`
		Change      []string `json:"change"`
		OccasionFit string   `json:"occasion_fit"`
		Swaps       []struct {
			InsteadOf string `json:"instead_of"`
			ItemID    int64  `json:"item_id"`
			Why       string `json:"why"`
		} `json:"swaps"`
	}
	if err := s.aiAsk(ctx, uid, st, aiVoice, prompt, imgs, &out); err != nil {
		s.userFail(w, err, "AI")
		return
	}
	own := map[int64]*store.Item{}
	for _, it := range items {
		own[it.ID] = it
	}
	swaps := []map[string]any{}
	for _, sw := range out.Swaps {
		m := map[string]any{"instead_of": clip(sw.InsteadOf, 80), "why": clip(sw.Why, 300)}
		if it := own[sw.ItemID]; it != nil {
			m["item"] = decorateItem(it)
		}
		swaps = append(swaps, m)
	}
	if out.Score < 1 || out.Score > 10 {
		out.Score = 0
	}
	writeJSON(w, http.StatusOK, map[string]any{"score": out.Score, "headline": clip(out.Headline, 200), "good": clipList(out.Good, 3, 240), "change": clipList(out.Change, 3, 240),
		"swaps": swaps, "occasion_fit": clip(out.OccasionFit, 240)})
}

func clipList(in []string, n, size int) []string {
	out := []string{}
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" && len(out) < n {
			out = append(out, clip(v, size))
		}
	}
	return out
}

// ---- your style and your colours ----

func (s *Server) handleAIFindStyle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		About string `json:"about"`
	}
	_ = decode(r, &in)
	st, done, ok := s.aiStart(w, r)
	if !ok {
		return
	}
	defer done()
	items, _ := s.store.ListItems(ctx, uid, store.ItemFilter{})
	if len(items) < 5 {
		writeErr(w, http.StatusBadRequest, "add at least five pieces first")
		return
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Wears > items[j].Wears })
	prompt := s.personText(st)
	if a := strings.TrimSpace(in.About); a != "" {
		prompt += "They say about themselves: " + clip(a, 600) + "\n"
	}
	prompt += "\nTheir wardrobe, most worn first:\n" + closetText(items, false) + `
From what they own and what they actually wear, describe their style.
Answer {"summary": "two or three sentences", "styles": [ids from: ` + styleIDs() + `] (1–3, best first), "palette": [colour families from: ` + familyNames() + `] (4–6 they wear most and that work),
"strengths": ["what their wardrobe does well", …3], "try": ["a concrete idea to freshen it", …3], "let_go": ["what they could wear less or let go", …2]}`
	var out struct {
		Summary   string   `json:"summary"`
		Styles    []string `json:"styles"`
		Palette   []string `json:"palette"`
		Strengths []string `json:"strengths"`
		Try       []string `json:"try"`
		LetGo     []string `json:"let_go"`
	}
	if err := s.aiAsk(ctx, uid, st, aiVoice, prompt, nil, &out); err != nil {
		s.userFail(w, err, "AI")
		return
	}
	styles := []string{}
	for _, id := range out.Styles {
		if _, ok := style.StyleByID(id); ok && len(styles) < 3 {
			styles = append(styles, id)
		}
	}
	palette := []string{}
	for _, f := range out.Palette {
		if _, ok := style.FamilyByName(f); ok && len(palette) < 6 {
			palette = append(palette, strings.ToLower(f))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"summary": clip(out.Summary, 800), "styles": styles, "palette": palette, "strengths": clipList(out.Strengths, 3, 240),
		"try": clipList(out.Try, 3, 240), "let_go": clipList(out.LetGo, 2, 240)})
}

func (s *Server) handleAIColours(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		PhotoID int64 `json:"photo_id"`
	}
	if err := decode(r, &in); err != nil || in.PhotoID == 0 {
		writeErr(w, http.StatusBadRequest, "send a photo of your face in daylight first")
		return
	}
	st, done, ok := s.aiStart(w, r)
	if !ok {
		return
	}
	defer done()
	imgs, err := s.photoForAI(ctx, uid, in.PhotoID)
	defer func() { _ = s.store.DeletePhotoIfUnused(ctx, uid, in.PhotoID) }() // never kept
	if err != nil {
		s.fail(w, err, "photo")
		return
	}
	prompt := `Do a simple seasonal colour analysis from this photo of a person's face (skin, eyes, hair) for choosing clothes.
Say how sure you can be from one photo; daylight and no make-up work best.
Answer {"season": one of [spring, summer, autumn, winter], "undertone": "warm, cool or neutral", "summary": "two sentences in plain words",
"best": [colour families from: ` + familyNames() + `] (6–8 that flatter them), "avoid": [colour families] (2–4), "metals": "gold, silver or both", "confidence": "low, medium or high"}`
	var out struct {
		Season     string   `json:"season"`
		Undertone  string   `json:"undertone"`
		Summary    string   `json:"summary"`
		Best       []string `json:"best"`
		Avoid      []string `json:"avoid"`
		Metals     string   `json:"metals"`
		Confidence string   `json:"confidence"`
	}
	if err := s.aiAsk(ctx, uid, st, aiVoice, prompt, imgs, &out); err != nil {
		s.userFail(w, err, "AI")
		return
	}
	keep := func(list []string, n int) []string {
		res := []string{}
		for _, f := range list {
			if _, ok := style.FamilyByName(f); ok && len(res) < n {
				res = append(res, strings.ToLower(f))
			}
		}
		return res
	}
	if !oneOf(out.Season, style.Seasons) {
		out.Season = ""
	}
	writeJSON(w, http.StatusOK, map[string]any{"season": out.Season, "undertone": clip(out.Undertone, 30), "summary": clip(out.Summary, 600), "best": keep(out.Best, 8),
		"avoid": keep(out.Avoid, 4), "metals": clip(out.Metals, 30), "confidence": clip(out.Confidence, 20)})
}

// ---- what to buy ----

func (s *Server) handleAIShopping(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		Wish   string `json:"wish"`
		Budget string `json:"budget"`
	}
	_ = decode(r, &in)
	st, done, ok := s.aiStart(w, r)
	if !ok {
		return
	}
	defer done()
	items, _ := s.store.ListItems(ctx, uid, store.ItemFilter{})
	prompt := s.personText(st)
	if w := strings.TrimSpace(in.Wish); w != "" {
		prompt += "What they are looking for: " + clip(w, 500) + "\n"
	}
	if b := strings.TrimSpace(in.Budget); b != "" {
		prompt += "Budget: " + clip(b, 60) + "\n"
	}
	prompt += "\nTheir wardrobe:\n" + closetText(items, false) + `
Suggest 6 pieces worth buying that would make the most new outfits with what they already own and fit their style, without repeating what they have.
Answer {"ideas": [{"title": "short, e.g. Camel wool coat", "kind": one of [` + kindIDs() + `], "color_name": one of [` + familyNames() + `], "why": "one or two sentences, mention which of their pieces it goes with", "price_hint": "a sensible price range"}]}`
	var out struct {
		Ideas []struct {
			Title     string `json:"title"`
			Kind      string `json:"kind"`
			ColorName string `json:"color_name"`
			Why       string `json:"why"`
			PriceHint string `json:"price_hint"`
		} `json:"ideas"`
	}
	if err := s.aiAsk(ctx, uid, st, aiVoice, prompt, nil, &out); err != nil {
		s.userFail(w, err, "AI")
		return
	}
	pieces, byID := s.pieces(ctx, uid, today())
	c := s.baseContext(ctx, uid, "")
	c.Season = ""
	res := []map[string]any{}
	used := map[int64]*store.Item{}
	for _, idea := range out.Ideas {
		k, ok := style.KindByID(idea.Kind)
		fam := strings.ToLower(idea.ColorName)
		if _, okc := style.FamilyByName(fam); !ok || !okc || len(res) >= 6 {
			continue
		}
		v := style.Virtual(k.ID, fam, st.Styles)
		pool := append(append([]style.Piece{}, pieces...), v)
		n, ex := style.Count(pool, c, 70, -1, 2)
		for _, e := range ex {
			for _, id := range e.Items {
				if it := byID[id]; it != nil {
					used[id] = decorateItem(it)
				}
			}
		}
		if ex == nil {
			ex = []style.Score{}
		}
		g := style.Gap{Title: clip(idea.Title, 120), Category: k.Category, Kind: k.ID, Family: fam, Hex: style.HexOf(fam), Why: clip(idea.Why, 500), Unlocks: n, Examples: ex}
		res = append(res, map[string]any{"gap": g, "price_hint": clip(idea.PriceHint, 60), "shops": searchLinks(st, searchWords(st, fam, k.ID, idea.Title))})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ideas": res, "items": used})
}

// handleAICheck reads a photo (or a shop page's picture) of something the
// person is thinking of buying, then the engine says how it fits in.
func (s *Server) handleAICheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		PhotoID int64  `json:"photo_id"`
		Title   string `json:"title"`
	}
	if err := decode(r, &in); err != nil || (in.PhotoID == 0 && strings.TrimSpace(in.Title) == "") {
		writeErr(w, http.StatusBadRequest, "send a photo, or say what it is")
		return
	}
	st, done, ok := s.aiStart(w, r)
	if !ok {
		return
	}
	defer done()
	imgs, err := s.photoForAI(ctx, uid, in.PhotoID)
	if err != nil {
		s.fail(w, err, "photo")
		return
	}
	prompt := "Someone is thinking of buying this piece. Describe it so a wardrobe app can check it against what they own. Answer one JSON object: " + pieceSchema
	if t := strings.TrimSpace(in.Title); t != "" {
		prompt += "\nThe shop calls it: " + clip(t, 200)
	}
	var p aiPiece
	if err := s.aiAsk(ctx, uid, st, aiVoice, prompt, imgs, &p); err != nil {
		s.userFail(w, err, "AI")
		return
	}
	p.clean()
	writeJSON(w, http.StatusOK, map[string]any{"item": p})
}

// handleAIPack has the AI pick a trip's clothes from the wardrobe.
func (s *Server) handleAIPack(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPack(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "packing")
		return
	}
	st, done, ok := s.aiStart(w, r)
	if !ok {
		return
	}
	defer done()
	items, _ := s.store.ListItems(ctx, uid, store.ItemFilter{})
	days := 3
	if p.StartDate != "" && p.EndDate != "" {
		days = daysBetween(p.StartDate, p.EndDate) + 1
	}
	prompt := s.personText(st) + fmt.Sprintf("Trip: %s to %s, %d days, %s.\n", p.Name, firstNonEmpty(p.Destination, "somewhere"), days, p.Purpose)
	if len(p.Weather) > 0 {
		prompt += "Forecast there: " + string(p.Weather) + "\n"
	}
	if p.Notes != "" {
		prompt += "Plans: " + clip(p.Notes, 600) + "\n"
	}
	prompt += "\nTheir wardrobe:\n" + closetText(items, false) + `
Pick a small capsule from these pieces (by id) that mixes into many outfits for this trip, plus the extras to bring (underwear, socks, swimwear, toiletries are not pieces).
Answer {"item_ids": [ids], "extras": ["Socks × 5", …], "tip": "one packing tip"}`
	var out struct {
		ItemIDs []int64  `json:"item_ids"`
		Extras  []string `json:"extras"`
		Tip     string   `json:"tip"`
	}
	if err := s.aiAsk(ctx, uid, st, aiVoice, prompt, nil, &out); err != nil {
		s.userFail(w, err, "AI")
		return
	}
	own, _ := s.store.OwnIDs(ctx, uid, out.ItemIDs)
	writeJSON(w, http.StatusOK, map[string]any{"item_ids": own, "extras": clipList(out.Extras, 20, 80), "tip": clip(out.Tip, 300)})
}
