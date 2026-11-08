package server

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/style"
)

// ---- what is worth buying ----

// searchLinks are "look for it" addresses in the person's shops.
func searchLinks(st store.Settings, words string) []map[string]string {
	shops := st.Shops
	if len(shops) == 0 {
		shops = store.DefaultShops
	}
	out := []map[string]string{}
	for _, sh := range shops {
		if !strings.Contains(sh.URL, "{q}") {
			continue
		}
		out = append(out, map[string]string{"name": sh.Name, "url": strings.ReplaceAll(sh.URL, "{q}", url.QueryEscape(words))})
	}
	return out
}

// searchWords is what to type in a shop for an idea ("navy chinos men").
func searchWords(st store.Settings, family, kind, title string) string {
	k, ok := style.KindByID(kind)
	w := strings.ToLower(title)
	if ok {
		w = family + " " + strings.ToLower(k.Label)
	}
	switch st.Presentation {
	case "menswear":
		w += " men"
	case "womenswear":
		w += " women"
	}
	return strings.TrimSpace(w)
}

// handleGaps works out the ideas from the wardrobe's gaps, for the
// person's styles (or one style), with example outfits and shop links.
func (s *Server) handleGaps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	st := s.settingsOf(ctx, uid)
	styles := st.Styles
	if v := r.URL.Query().Get("style"); v != "" {
		if _, ok := style.StyleByID(v); ok {
			styles = []string{v}
		}
	}
	pieces, items := s.pieces(ctx, uid, today())
	c := s.baseContext(ctx, uid, "")
	c.Season = ""
	gaps := style.Gaps(pieces, c, styles, 10)
	kept, _ := s.store.ListIdeas(ctx, uid, "")
	used := map[int64]*store.Item{}
	out := []map[string]any{}
	for _, g := range gaps {
		var status string
		var keptID int64
		for _, k := range kept {
			if k.Kind == g.Kind && k.ColorName == g.Family && k.MadeBy == "gaps" {
				status, keptID = k.Status, k.ID
			}
		}
		if status == "dismissed" || status == "bought" {
			continue
		}
		for _, ex := range g.Examples {
			for _, id := range ex.Items {
				if it := items[id]; it != nil {
					used[id] = decorateItem(it)
				}
			}
		}
		out = append(out, map[string]any{"gap": g, "status": status, "idea_id": keptID, "shops": searchLinks(st, searchWords(st, g.Family, g.Kind, g.Title))})
	}
	names := []string{}
	for _, sid := range styles {
		if sv, ok := style.StyleByID(sid); ok {
			names = append(names, sv.Name)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ideas": out, "items": used, "styles": styles, "style_names": names, "pieces": len(pieces)})
}

type ideaInput struct {
	Title     *string `json:"title"`
	Category  *string `json:"category"`
	Kind      *string `json:"kind"`
	ColorHex  *string `json:"color_hex"`
	ColorName *string `json:"color_name"`
	Why       *string `json:"why"`
	Style     *string `json:"style"`
	Unlocks   *int    `json:"unlocks"`
	PriceHint *string `json:"price_hint"`
	URL       *string `json:"url"`
	PhotoID   *int64  `json:"photo_id"`
	Status    *string `json:"status"`
	MadeBy    *string `json:"made_by"`
}

func (s *Server) applyIdea(r *http.Request, in ideaInput, i *store.Idea) string {
	t := func(p *string, n int) string { return clip(strings.TrimSpace(*p), n) }
	if in.Title != nil {
		i.Title = t(in.Title, 160)
	}
	if in.Kind != nil {
		if *in.Kind != "" {
			k, ok := style.KindByID(*in.Kind)
			if !ok {
				return "unknown kind of piece"
			}
			i.Category = k.Category
		}
		i.Kind = *in.Kind
	}
	if in.Category != nil && i.Kind == "" {
		i.Category = t(in.Category, 20)
	}
	if in.ColorName != nil {
		i.ColorName = strings.ToLower(t(in.ColorName, 30))
		if f, ok := style.FamilyByName(i.ColorName); ok && i.ColorHex == "" {
			i.ColorHex = f.Hex
		}
	}
	if in.ColorHex != nil {
		if *in.ColorHex != "" && !style.ValidHex(*in.ColorHex) {
			return "the colour is #rrggbb"
		}
		i.ColorHex = strings.ToLower(*in.ColorHex)
		if i.ColorName == "" && i.ColorHex != "" {
			i.ColorName = style.FamilyOf(i.ColorHex)
		}
	}
	if in.Why != nil {
		i.Why = t(in.Why, 600)
	}
	if in.Style != nil {
		i.Style = t(in.Style, 30)
	}
	if in.Unlocks != nil && *in.Unlocks >= 0 {
		i.Unlocks = *in.Unlocks
	}
	if in.PriceHint != nil {
		i.PriceHint = t(in.PriceHint, 60)
	}
	if in.URL != nil {
		v := strings.TrimSpace(*in.URL)
		if v != "" {
			if c, ok := cleanURL(v); ok {
				v = c
			} else {
				return "the link does not look right"
			}
		}
		i.URL = v
	}
	if in.Status != nil {
		if !oneOf(*in.Status, store.IdeaStates) {
			return "unknown state"
		}
		i.Status = *in.Status
	}
	if i.Title == "" {
		return "say what it is"
	}
	return ""
}

func (s *Server) handleListIdeas(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	list, err := s.store.ListIdeas(ctx, actorID(ctx), r.URL.Query().Get("status"))
	if err != nil {
		s.fail(w, err, "ideas")
		return
	}
	st := s.settingsOf(ctx, actorID(ctx))
	out := []map[string]any{}
	for _, i := range list {
		i.PhotoURL, _ = photoURL(i.PhotoID)
		out = append(out, map[string]any{"idea": i, "shops": searchLinks(st, searchWords(st, i.ColorName, i.Kind, i.Title))})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ideas": out})
}

func (s *Server) handleCreateIdea(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in ideaInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	i := &store.Idea{UserID: uid, MadeBy: "gaps", Status: "idea"}
	if in.MadeBy != nil && oneOf(*in.MadeBy, []string{"gaps", "ai", "check"}) {
		i.MadeBy = *in.MadeBy
	}
	if msg := s.applyIdea(r, in, i); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if id, ok := s.photoMine(ctx, uid, in.PhotoID); !ok {
		writeErr(w, http.StatusBadRequest, "unknown photo")
		return
	} else {
		i.PhotoID = id
	}
	if err := s.store.CreateIdea(ctx, i); err != nil {
		s.fail(w, err, "idea")
		return
	}
	s.publish(ctx, uid, "wardrobe.idea_saved", "idea:"+itoa(i.ID), map[string]any{"id": i.ID, "title": i.Title, "unlocks": i.Unlocks})
	writeJSON(w, http.StatusCreated, i)
}

func (s *Server) handleUpdateIdea(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	i, err := s.store.GetIdea(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "idea")
		return
	}
	var in ideaInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if msg := s.applyIdea(r, in, i); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if err := s.store.UpdateIdea(ctx, i); err != nil {
		s.fail(w, err, "idea")
		return
	}
	writeJSON(w, http.StatusOK, i)
}

func (s *Server) handleDeleteIdea(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeleteIdea(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "idea")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// handleIdeaWishlist puts an idea on Pocket Wishlist.
func (s *Server) handleIdeaWishlist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	i, err := s.store.GetIdea(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "idea")
		return
	}
	st := s.settingsOf(ctx, uid)
	notes := strings.TrimSpace(i.Why)
	if i.Unlocks > 0 {
		notes += "\n\nMakes about " + itoa(int64(i.Unlocks)) + " new outfits with what is in the wardrobe."
	}
	if links := searchLinks(st, searchWords(st, i.ColorName, i.Kind, i.Title)); len(links) > 0 && i.URL == "" {
		notes += "\n\nLook for it: " + links[0]["url"]
	}
	c := s.clients(ctx, uid)
	res, sib, err := s.toWishlist(ctx, c, i.Title, i.URL, notes, "", nil)
	if err != nil {
		s.userFail(w, err, "wishlist")
		return
	}
	i.Status, i.WishRef, i.WishURL = "wish", idOf(res), linkTo(sib, "/wishes/"+idOf(res))
	_ = s.store.UpdateIdea(ctx, i)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "idea": i, "url": i.WishURL, "text": "On your wishlist"})
}

// handleIdeaBought turns an idea into a piece in the wardrobe.
func (s *Server) handleIdeaBought(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	i, err := s.store.GetIdea(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "idea")
		return
	}
	st := s.settingsOf(ctx, uid)
	k, _ := style.KindByID(i.Kind)
	it := &store.Item{UserID: uid, Name: i.Title, Category: firstNonEmpty(i.Category, k.Category, "other"), Kind: i.Kind, Pattern: "plain", Warmth: k.Warmth, Formality: k.Formality, Rain: k.Rain,
		Status: "ready", Currency: st.Currency, BoughtOn: today(), Source: "idea", SourceRef: itoa(i.ID), PhotoID: i.PhotoID, Seasons: []string{}, Styles: []string{}}
	if i.Style != "" {
		it.Styles = []string{i.Style}
	}
	if i.ColorHex != "" {
		it.Colors = []store.Color{{Hex: i.ColorHex, Name: firstNonEmpty(i.ColorName, style.FamilyOf(i.ColorHex))}}
	}
	if it.Kind == "" {
		it.Kind = "other"
	}
	if err := s.store.CreateItem(ctx, it); err != nil {
		s.fail(w, err, "item")
		return
	}
	i.Status = "bought"
	_ = s.store.UpdateIdea(ctx, i)
	s.publish(ctx, uid, "wardrobe.item_added", "item:"+itoa(it.ID), map[string]any{"id": it.ID, "name": it.Name, "category": it.Category, "kind": it.Kind, "url": s.link("/closet/" + itoa(it.ID))})
	writeJSON(w, http.StatusOK, map[string]any{"item": it})
}

// ---- before you buy ----

// handleCheck answers "should I buy this?": a piece described by hand,
// by the AI from a photo, or read from a shop page.
func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		Kind      string `json:"kind"`
		ColorHex  string `json:"color_hex"`
		ColorName string `json:"color_name"`
		Pattern   string `json:"pattern"`
		Formality int    `json:"formality"`
		Warmth    int    `json:"warmth"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	k, ok := style.KindByID(in.Kind)
	if !ok {
		writeErr(w, http.StatusBadRequest, "say what kind of piece it is")
		return
	}
	hex := strings.ToLower(in.ColorHex)
	if !style.ValidHex(hex) {
		hex = style.HexOf(in.ColorName)
	}
	p := style.Virtual(k.ID, style.FamilyOf(hex), nil)
	p.Colors = []string{hex}
	if oneOf(in.Pattern, style.Patterns) {
		p.Pattern = in.Pattern
	}
	if in.Formality >= 1 && in.Formality <= 5 {
		p.Formality = in.Formality
	}
	if in.Warmth >= 1 && in.Warmth <= 5 {
		p.Warmth = in.Warmth
	}
	st := s.settingsOf(ctx, uid)
	pieces, items := s.pieces(ctx, uid, today())
	c := s.baseContext(ctx, uid, "")
	c.Season = ""
	v := style.Check(pieces, p, c, st.Styles)
	used := map[int64]*store.Item{}
	for _, id := range v.Similar {
		if it := items[id]; it != nil {
			used[id] = decorateItem(it)
		}
	}
	for _, ex := range v.Examples {
		for _, id := range ex.Items {
			if it := items[id]; it != nil {
				used[id] = decorateItem(it)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"verdict": v, "items": used, "kind": k, "color_hex": hex})
}

// handlePreview reads a shop page for its name, picture and price.
func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		URL string `json:"url"`
	}
	if err := decode(r, &in); err != nil || strings.TrimSpace(in.URL) == "" {
		writeErr(w, http.StatusBadRequest, "paste a shop link")
		return
	}
	if s.cfg.DemoData {
		writeJSON(w, http.StatusOK, map[string]any{"title": "Wool-blend overshirt", "image": "", "price": 5990, "currency": "EUR", "shop": "Example shop", "via": "demo"})
		return
	}
	res, err := s.productPage(ctx, s.clients(ctx, actorID(ctx)), strings.TrimSpace(in.URL))
	if err != nil {
		s.userFail(w, err, "shop page")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleShopLinks is "look for it" for any words.
func (s *Server) handleShopLinks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st := s.settingsOf(ctx, actorID(ctx))
	q := r.URL.Query()
	words := strings.TrimSpace(q.Get("q"))
	if words == "" {
		words = searchWords(st, q.Get("color"), q.Get("kind"), q.Get("title"))
	}
	writeJSON(w, http.StatusOK, map[string]any{"shops": searchLinks(st, words), "words": words})
}
