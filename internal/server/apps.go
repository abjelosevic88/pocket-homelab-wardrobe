package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/style"
)

// ---- the wardrobe in the other Pocket apps ----
//
// As the person (their own Core link or direct token): the weather from
// Pocket Weather (weather.go), the day's events from Pocket Calendar,
// training days from Pocket Fitness, hikes from Pocket Trails, trips from
// Pocket Travel, clothes bought in Pocket Money, clothes on their way in
// Pocket Parcels, wishes in Pocket Wishlist, reminders in Pocket Tasks,
// "what do you think?" in Pocket Chat and notes in Pocket Notes. None of
// it is needed: when an app is not there, the page greys the button and
// says what to do.

func linkTo(sib *siblingConn, path string) string {
	if sib == nil || sib.public == "" {
		return ""
	}
	return strings.TrimRight(sib.public, "/") + path
}

func anyList(v any) []any {
	a, _ := v.([]any)
	return a
}

// plainErr is an app's error in a line.
func plainErr(a pocketApp, err error) string {
	if isNotConnected(err) {
		return err.Error()
	}
	return a.name + ": " + err.Error()
}

// dayEvent is a calendar event and what to wear for it.
type dayEvent struct {
	Title    string `json:"title"`
	Time     string `json:"time"`
	EndTime  string `json:"end_time,omitempty"`
	AllDay   bool   `json:"all_day,omitempty"`
	Place    string `json:"place,omitempty"`
	Occasion string `json:"occasion"`
	Ref      string `json:"ref"`
	URL      string `json:"url,omitempty"`
}

// dayEvents are the person's events on a day in Pocket Calendar, each
// with the occasion it most likely needs.
func (s *Server) dayEvents(ctx context.Context, c *clients, date string) ([]dayEvent, error) {
	if s.cfg.DemoData {
		return demoEvents(date), nil
	}
	sib, err := c.conn(ctx, appCalendar, scopeCalendarRead)
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	v, err := getJSONFrom(cctx, sib, "/api/v1/events", url.Values{"from": {date}, "to": {store.AddDays(date, 1)}, "tz": {time.Local.String()}})
	if err != nil {
		return nil, err
	}
	out := []dayEvent{}
	for _, m := range rows(v, "events") {
		start := str(m, "start")
		if day10(start) != date && !(m["all_day"] == true && day10(start) <= date) {
			continue
		}
		e := dayEvent{Title: firstNonEmpty(str(m, "title"), "Busy"), Place: str(m, "location"), Ref: idOf(m), URL: linkTo(sib, "/event/"+url.PathEscape(idOf(m)))}
		if m["all_day"] == true {
			e.AllDay = true
		} else {
			e.Time, e.EndTime = clock(start), clock(str(m, "end"))
		}
		e.Occasion = style.OccasionFor(e.Title + " " + e.Place)
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time < out[j].Time })
	return out, nil
}

// clock is the HH:MM of a time ("" for a date).
func clock(v string) string {
	if len(v) >= 16 && v[10] == 'T' {
		return v[11:16]
	}
	return ""
}

// occasionOf is what the day as a whole should be dressed for: the
// smartest thing on it.
func occasionOf(events []dayEvent, weekday time.Weekday) (string, string) {
	best, why := "", ""
	bestF := -1.0
	for _, e := range events {
		if e.Occasion == "" || e.Occasion == "sport" {
			continue
		}
		if f := style.OccasionByID(e.Occasion).Formality; f > bestF {
			best, why, bestF = e.Occasion, e.Title, f
		}
	}
	if best == "" && (weekday == time.Saturday || weekday == time.Sunday) {
		return "everyday", ""
	}
	return best, why
}

// training is a planned workout on a day in Pocket Fitness.
type training struct {
	Name string `json:"name"`
	Time string `json:"time,omitempty"`
	Done bool   `json:"done"`
	URL  string `json:"url,omitempty"`
}

func (s *Server) trainingOn(ctx context.Context, c *clients, date string) (*training, error) {
	if s.cfg.DemoData {
		if t, _ := store.ParseDate(date); t.Weekday() == time.Monday || t.Weekday() == time.Thursday {
			return &training{Name: "Upper body", Time: "18:30"}, nil
		}
		return nil, nil
	}
	sib, err := c.conn(ctx, appFitness, scopeFitnessRead)
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	v, err := getJSONFrom(cctx, sib, "/api/v1/week", url.Values{"date": {date}})
	if err != nil {
		return nil, err
	}
	m, _ := v.(map[string]any)
	for _, d := range rows(m["days"]) {
		if str(d, "date") != date {
			continue
		}
		if p := obj(d, "planned"); p != nil {
			return &training{Name: firstNonEmpty(str(p, "routine"), "Training"), Time: str(p, "time"), Done: len(rows(d["items"])) > 0, URL: linkTo(sib, "/")}, nil
		}
		if items := rows(d["items"]); len(items) > 0 {
			return &training{Name: firstNonEmpty(str(items[0], "name"), "Training"), Done: true, URL: linkTo(sib, "/")}, nil
		}
	}
	return nil, nil
}

// outdoor is a hike or a ride on a day in Pocket Trails.
type outdoor struct {
	Title    string `json:"title"`
	Activity string `json:"activity"`
	URL      string `json:"url,omitempty"`
}

func (s *Server) outingOn(ctx context.Context, c *clients, date string) (*outdoor, error) {
	if s.cfg.DemoData {
		if t, _ := store.ParseDate(date); t.Weekday() == time.Saturday {
			return &outdoor{Title: "Trebević loop", Activity: "hike"}, nil
		}
		return nil, nil
	}
	sib, err := c.conn(ctx, appTrails, scopeTrailsRead)
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	v, err := getJSONFrom(cctx, sib, "/api/v1/today", nil)
	if err != nil {
		return nil, err
	}
	m, _ := v.(map[string]any)
	list := rows(m["upcoming"])
	if cur := obj(m, "current"); cur != nil {
		list = append([]map[string]any{cur}, list...)
	}
	for _, o := range list {
		if str(o, "date") == date && str(o, "status") != "cancelled" {
			return &outdoor{Title: firstNonEmpty(str(o, "title"), str(o, "route_name"), "Outing"), Activity: str(o, "activity"), URL: linkTo(sib, "/outings/"+idOf(o))}, nil
		}
	}
	return nil, nil
}

// trip is a trip in Pocket Travel.
type trip struct {
	Ref         string   `json:"ref"`
	Name        string   `json:"name"`
	Destination string   `json:"destination"`
	Start       string   `json:"start_date"`
	End         string   `json:"end_date"`
	Lat         *float64 `json:"lat"`
	Lng         *float64 `json:"lng"`
	URL         string   `json:"url,omitempty"`
}

func (s *Server) trips(ctx context.Context, c *clients) ([]trip, error) {
	if s.cfg.DemoData {
		lat, lng := 41.3874, 2.1686
		return []trip{{Ref: "7", Name: "Barcelona", Destination: "Barcelona, Spain", Start: store.AddDays(today(), 12), End: store.AddDays(today(), 16), Lat: &lat, Lng: &lng}}, nil
	}
	sib, err := c.conn(ctx, appTravel, scopeTripsRead)
	if err != nil {
		return nil, err
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/trips", nil)
	if err != nil {
		return nil, err
	}
	out := []trip{}
	for _, m := range rows(v, "trips") {
		end := firstNonEmpty(str(m, "end_date"), str(m, "start_date"))
		if m["archived"] == true || (end != "" && end < today()) {
			continue
		}
		t := trip{Ref: idOf(m), Name: str(m, "name"), Destination: firstNonEmpty(str(m, "destination"), str(m, "name")), Start: str(m, "start_date"), End: end, URL: linkTo(sib, "/trips/"+idOf(m))}
		for _, k := range [][2]string{{"lat", "lng"}, {"dest_lat", "dest_lng"}, {"latitude", "longitude"}} {
			if a, b := fptr(m, k[0]), fptr(m, k[1]); a != nil && b != nil {
				t.Lat, t.Lng = a, b
				break
			}
		}
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out, nil
}

// ---- clothes you bought ----

var fashionShops = []string{"zara", "h&m", "mango", "c&a", "uniqlo", "primark", "bershka", "pull&bear", "pull & bear", "stradivarius", "massimo dutti", "arket",
	"zalando", "about you", "asos", "nike", "adidas", "puma", "deichmann", "reserved", "lc waikiki", "new yorker", "tommy hilfiger", "levi's", "levis", "s.oliver", "tally weijl", "sinsay",
	"decathlon", "intersport", "sport vision", "humanic", "vinted", "shein", "jack & jones", "vero moda", "lacoste", "ralph lauren",
	"benetton", "calzedonia", "intimissimi", "tezenis", "geox", "ecco", "timberland", "the north face", "patagonia", "skechers", "converse", "vans", "new balance", "hugo boss", "guess", "cos stores"}

var fashionWords = []string{"clothing", "clothes", "fashion", "apparel", "shoes", "odjeća", "odjeca", "obuća", "obuca", "garderob", "kleidung", "schuhe", "jacket", "jakna", "dress", "haljin",
	"jeans", "farmerk", "shirt", "košulj", "kosulj", "sneaker", "patik", "boots", "čizm", "cizm", "kaput", "sweater", "džemper", "dzemper", "hoodie", "trousers", "hlače", "hlace", "pantalon",
	"pidžam", "pidzam", "majic", "duks", "suknj", "cipel", "trenerk"}

// fashionWhy says why a purchase looks like clothing ("" = it does not):
// a shop's name or a clothing word, as a whole word.
func fashionWhy(parts ...string) string {
	t := " " + strings.ToLower(strings.Join(parts, " ")) + " "
	word := func(k string) bool {
		i := strings.Index(t, k)
		for i >= 0 {
			before, after := t[i-1], byte(' ')
			if i+len(k) < len(t) {
				after = t[i+len(k)]
			}
			if !isLetter(before) && (!isLetter(after) || len(k) >= 5) {
				return true
			}
			j := strings.Index(t[i+1:], k)
			if j < 0 {
				break
			}
			i += 1 + j
		}
		return false
	}
	for _, k := range fashionShops {
		if word(k) {
			return k
		}
	}
	for _, k := range fashionWords {
		if word(k) {
			return k
		}
	}
	return ""
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 0x80 }

func looksLikeFashion(parts ...string) bool { return fashionWhy(parts...) != "" }

// purchase is a clothing buy in Pocket Money or Pocket Parcels.
type purchase struct {
	App        string `json:"app"` // money | parcels
	Ref        string `json:"ref"`
	Date       string `json:"date"`
	Title      string `json:"title"`
	Shop       string `json:"shop"`
	Amount     *int64 `json:"amount"`
	Currency   string `json:"currency"`
	Status     string `json:"status,omitempty"`
	ExpectedOn string `json:"expected_on,omitempty"`
	ReturnBy   string `json:"return_by,omitempty"`
	URL        string `json:"url,omitempty"`
	Added      int64  `json:"added"` // the piece it became, 0 = not yet
	Why        string `json:"why"`   // the shop or the word that made it look like clothing
}

func (s *Server) moneyPurchases(ctx context.Context, c *clients, days int) ([]purchase, error) {
	sib, err := c.conn(ctx, appMoney, scopeTxRead)
	if err != nil {
		return nil, err
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/transactions", url.Values{"kind": {"expense"}, "from": {store.AddDays(today(), -days)}, "limit": {"500"}})
	if err != nil {
		return nil, err
	}
	out := []purchase{}
	for _, m := range rows(v, "items") {
		why := fashionWhy(str(m, "category_name"), str(m, "payee"), str(m, "description"))
		if why == "" {
			continue
		}
		amt := int64(num(m, "amount"))
		if amt < 0 {
			amt = -amt
		}
		out = append(out, purchase{App: "money", Ref: idOf(m), Date: str(m, "date"), Title: firstNonEmpty(str(m, "description"), str(m, "payee")), Shop: str(m, "payee"),
			Amount: &amt, Currency: str(m, "currency"), URL: linkTo(sib, "/transactions/"+idOf(m)), Why: why})
	}
	return out, nil
}

func (s *Server) parcelPurchases(ctx context.Context, c *clients) ([]purchase, error) {
	sib, err := c.conn(ctx, appParcels, scopeParcelsRead)
	if err != nil {
		return nil, err
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/parcels", url.Values{"limit": {"200"}})
	if err != nil {
		return nil, err
	}
	out := []purchase{}
	for _, m := range rows(v, "parcels") {
		if str(m, "direction") == "outgoing" || !looksLikeFashion(str(m, "shop"), str(m, "title"), str(m, "notes")) {
			continue
		}
		if m["archived"] == true && str(m, "return_by") < today() {
			continue
		}
		p := purchase{App: "parcels", Ref: idOf(m), Date: firstNonEmpty(day10(str(m, "delivered_at")), str(m, "ordered_on")), Title: firstNonEmpty(str(m, "title"), str(m, "shop")), Shop: str(m, "shop"),
			Currency: str(m, "currency"), Status: str(m, "status"), ExpectedOn: str(m, "expected_on"), URL: linkTo(sib, "/parcels/"+idOf(m))}
		if str(m, "return_state") == "" || str(m, "return_state") == "none" {
			p.ReturnBy = str(m, "return_by")
		}
		if v, ok := m["price"].(float64); ok {
			a := int64(v)
			p.Amount = &a
		}
		out = append(out, p)
	}
	return out, nil
}

// handlePurchases lists clothes bought (Pocket Money) and on their way or
// just arrived (Pocket Parcels), with which are already in the wardrobe.
func (s *Server) handlePurchases(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	c := s.clients(ctx, uid)
	var mu sync.Mutex
	var wg sync.WaitGroup
	out := map[string]any{"purchases": []purchase{}}
	all := []purchase{}
	errs := map[string]string{}
	run := func(a pocketApp, fn func() ([]purchase, error)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			list, err := fn()
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs[a.slug] = plainErr(a, err)
				return
			}
			all = append(all, list...)
		}()
	}
	if s.cfg.DemoData {
		all = demoPurchases()
	} else {
		run(appMoney, func() ([]purchase, error) { return s.moneyPurchases(ctx, c, 120) })
		run(appParcels, func() ([]purchase, error) { return s.parcelPurchases(ctx, c) })
		wg.Wait()
	}
	for i := range all {
		if list, _ := s.store.ListItems(ctx, uid, store.ItemFilter{Source: all[i].App, Ref: all[i].Ref}); len(list) > 0 {
			all[i].Added = list[0].ID
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Date > all[j].Date })
	out["purchases"], out["errors"] = all, errs
	writeJSON(w, http.StatusOK, out)
}

// ---- Pocket Wishlist ----

// toWishlist puts something worth buying on the person's wishlist.
func (s *Server) toWishlist(ctx context.Context, c *clients, title, link, notes, imageURL string, price *int64) (map[string]any, *siblingConn, error) {
	sib, err := c.conn(ctx, appWishlist, scopeWishesWrite)
	if err != nil {
		return nil, nil, err
	}
	body := map[string]any{"title": clip(title, 200), "notes": clip(notes, 2000), "tags": "clothes", "no_lookup": link == ""}
	if link != "" {
		body["url"] = link
	}
	if imageURL != "" {
		body["image_url"] = imageURL
	}
	if price != nil {
		body["price"] = *price
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/wishes", body)
	return res, sib, err
}

// productPage reads a shop page for its name, picture and price — through
// Pocket Wishlist (which knows shops), else directly when online is on.
func (s *Server) productPage(ctx context.Context, c *clients, link string) (map[string]any, error) {
	if sib, err := optional(c.conn(ctx, appWishlist, scopeWishesWrite)); err == nil && sib != nil {
		res, err := postJSONTo(ctx, sib, "/api/v1/preview", map[string]any{"url": link})
		if err == nil {
			return map[string]any{"title": firstNonEmpty(str(res, "title"), str(res, "name")), "image": firstNonEmpty(str(res, "image_url"), str(res, "image")),
				"price": res["price"], "currency": str(res, "currency"), "shop": firstNonEmpty(str(res, "store"), str(res, "site")), "via": "wishlist"}, nil
		}
	}
	if !s.online(c.st) {
		return nil, &userErr{"connect Pocket Wishlist (or switch on looking things up online) to read shop pages"}
	}
	return s.readProductPage(ctx, link)
}

// handleWishes lists clothing wishes in Pocket Wishlist ("before you buy" checks them).
func (s *Server) handleWishes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c := s.clients(ctx, actorID(ctx))
	if s.cfg.DemoData {
		writeJSON(w, http.StatusOK, map[string]any{"wishes": demoWishes()})
		return
	}
	sib, err := c.conn(ctx, appWishlist, scopeWishesRead)
	if err != nil {
		s.userFail(w, err, "wishlist")
		return
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/wishes", url.Values{"limit": {"200"}})
	if err != nil {
		writeErr(w, http.StatusBadGateway, plainErr(appWishlist, err))
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "wishes") {
		if st := str(m, "status"); st == "bought" || st == "archived" {
			continue
		}
		if !looksLikeFashion(str(m, "title"), str(m, "store"), str(m, "tags"), str(m, "notes")) && !strings.Contains(strings.ToLower(str(m, "tags")), "clothes") {
			continue
		}
		out = append(out, map[string]any{"ref": idOf(m), "title": str(m, "title"), "store": str(m, "store"), "price": m["price"], "currency": str(m, "currency"),
			"image": firstNonEmpty(str(m, "image_url"), str(m, "photo_url")), "url": str(m, "url"), "open_url": linkTo(sib, "/wishes/"+idOf(m))})
	}
	writeJSON(w, http.StatusOK, map[string]any{"wishes": out})
}

// ---- Pocket Chat: "what do you think?" ----

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c := s.clients(ctx, actorID(ctx))
	sib, err := c.conn(ctx, appChat, scopeChatRead)
	if err != nil {
		s.userFail(w, err, "chat")
		return
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/conversations", nil)
	if err != nil {
		writeErr(w, http.StatusBadGateway, plainErr(appChat, err))
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "conversations") {
		if m["archived"] == true {
			continue
		}
		out = append(out, map[string]any{"id": idOf(m), "name": firstNonEmpty(str(m, "name"), str(m, "title")), "kind": str(m, "kind"), "emoji": str(m, "emoji")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": out})
}

// handleShareChat posts pieces (an outfit, a suggestion, a shop find) into
// a Pocket Chat conversation as a card with their picture.
func (s *Server) handleShareChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		To       string  `json:"to"`
		Text     string  `json:"text"`
		ItemIDs  []int64 `json:"item_ids"`
		OutfitID int64   `json:"outfit_id"`
		Title    string  `json:"title"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	ids, _ := s.store.OwnIDs(ctx, uid, in.ItemIDs)
	title := strings.TrimSpace(in.Title)
	link := s.link("/")
	if in.OutfitID > 0 {
		o, err := s.store.GetOutfit(ctx, uid, in.OutfitID)
		if err != nil {
			s.fail(w, err, "outfit")
			return
		}
		ids, title, link = o.ItemIDs, firstNonEmpty(title, o.Name), s.link("/outfits/"+itoa(o.ID))
	}
	if len(ids) == 0 {
		writeErr(w, http.StatusBadRequest, "pick what to show")
		return
	}
	items, _ := s.store.ItemsByIDs(ctx, uid, ids)
	var lines []string
	for _, id := range ids {
		if it := items[id]; it != nil && len(lines) < 8 {
			lines = append(lines, it.Name)
		}
	}
	if title == "" {
		title = s.outfitName(ctx, uid, ids)
	}
	c := s.clients(ctx, uid)
	sib, err := c.conn(ctx, appChat, scopeChatWrite)
	if err != nil {
		s.userFail(w, err, "chat")
		return
	}
	card := map[string]any{"app": "wardrobe", "kind": "outfit", "title": clip(title, 200), "subtitle": "What do you think?", "lines": lines, "image": s.shareURL(uid, ids), "url": link, "icon": "👗"}
	body := map[string]any{"to": firstNonEmpty(strings.TrimSpace(in.To), "me"), "text": clip(firstNonEmpty(strings.TrimSpace(in.Text), "What do you think of this?"), 2000), "card": card,
		"client_id": fmt.Sprintf("wardrobe-%d-%d", uid, time.Now().Unix()/30)}
	res, err := postJSONTo(ctx, sib, "/api/v1/send", body)
	if err != nil {
		writeErr(w, http.StatusBadGateway, plainErr(appChat, err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": linkTo(sib, "/c/"+str(res, "conversation_id")), "text": "Sent to Pocket Chat"})
}

// ---- Pocket Notes ----

func (s *Server) toNotes(ctx context.Context, c *clients, title, content string) (string, error) {
	sib, err := c.conn(ctx, appNotes, scopeNotesWrite)
	if err != nil {
		return "", err
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/notes", map[string]any{"title": clip(title, 200), "content": content, "category": "Wardrobe"})
	if err != nil {
		return "", fmt.Errorf("Pocket Notes: %w", err)
	}
	return linkTo(sib, "/notes/"+idOf(res)), nil
}

func (s *Server) handleSaveNote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := decode(r, &in); err != nil || strings.TrimSpace(in.Content) == "" {
		writeErr(w, http.StatusBadRequest, "nothing to save")
		return
	}
	u, err := s.toNotes(ctx, s.clients(ctx, actorID(ctx)), firstNonEmpty(strings.TrimSpace(in.Title), "Wardrobe"), clip(in.Content, 50000))
	if err != nil {
		s.userFail(w, err, "notes")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": u, "text": "Saved in Pocket Notes"})
}

// handleApps lists how the person reaches each app, in words they can act on.
func (s *Server) handleApps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c := s.clients(ctx, actorID(ctx))
	out := []appState{}
	for _, a := range pocketApps {
		out = append(out, c.status(ctx, a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": out, "core": s.core.Configured()})
}
