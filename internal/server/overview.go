package server

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/style"
)

// ---- today, and the overviews for the rest of the box ----

// dayPlan is everything that decides what to wear on a day.
type dayPlan struct {
	Date       string                `json:"date"`
	Weather    *dayWeather           `json:"weather"`
	WeatherWhy string                `json:"weather_why,omitempty"`
	Events     []dayEvent            `json:"events"`
	EventsWhy  string                `json:"events_why,omitempty"`
	Training   *training             `json:"training"`
	Outing     *outdoor              `json:"outing"`
	Occasion   string                `json:"occasion"`
	Because    string                `json:"because,omitempty"` // the event the occasion comes from
	Plan       *store.Wear           `json:"plan"`
	Worn       *store.Wear           `json:"worn"`
	Suggest    suggestOut            `json:"suggest"`
	Tips       []string              `json:"tips"`
	Items      map[int64]*store.Item `json:"items"`
}

// planDay works out a day: the weather, what is on, the plan or what was
// worn, and outfits for it.
func (s *Server) planDay(ctx context.Context, uid int64, date, occasion, styleID string, shuffle int64) dayPlan {
	c := s.clients(ctx, uid)
	out := dayPlan{Date: date, Events: []dayEvent{}, Tips: []string{}, Items: map[int64]*store.Item{}}
	var wg sync.WaitGroup
	var mu sync.Mutex
	if c.st.UseCalendar {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ev, err := s.dayEvents(ctx, c, date)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				out.EventsWhy = plainErr(appCalendar, err)
				return
			}
			out.Events = ev
		}()
	}
	if c.st.UseFitness {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t, err := s.trainingOn(ctx, c, date)
			if err == nil {
				mu.Lock()
				out.Training = t
				mu.Unlock()
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		o, err := s.outingOn(ctx, c, date)
		if err == nil {
			mu.Lock()
			out.Outing = o
			mu.Unlock()
		}
	}()
	wg.Wait()
	t, _ := store.ParseDate(date)
	occ, why := occasionOf(out.Events, t.Weekday())
	if occ == "" {
		occ = "work"
		if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
			occ = "everyday"
		}
	}
	if out.Outing != nil && occasion == "" {
		occ, why = "outdoors", out.Outing.Title
	}
	if occasion != "" {
		occ, why = occasion, ""
	}
	out.Occasion, out.Because = occ, why
	if p, err := s.store.PlanFor(ctx, uid, date); err == nil {
		out.Plan = s.decorateWear(p)
	}
	if list, _ := s.store.ListWears(ctx, uid, date, date); len(list) > 0 {
		for _, x := range list {
			if !x.Planned {
				out.Worn = s.decorateWear(x)
			}
		}
	}
	out.Suggest = s.suggest(ctx, uid, suggestInput{Date: date, Occasion: occ, Style: styleID, Shuffle: shuffle, N: 4})
	out.Weather, out.WeatherWhy = out.Suggest.Weather, out.Suggest.WeatherWhy
	for k, v := range out.Suggest.Items {
		out.Items[k] = v
	}
	var ids []int64
	for _, w := range []*store.Wear{out.Plan, out.Worn} {
		if w != nil {
			ids = append(ids, w.ItemIDs...)
		}
	}
	if m, _ := s.store.ItemsByIDs(ctx, uid, ids); m != nil {
		for k, v := range m {
			out.Items[k] = decorateItem(v)
		}
	}
	// a few plain words for the day
	w := out.Weather
	if w != nil && w.Rainy() {
		out.Tips = append(out.Tips, "Rain is likely: a rain jacket or an umbrella, and shoes that do not mind water.")
	}
	if w != nil && w.Snowy() {
		out.Tips = append(out.Tips, "Snow: boots with grip and a warm coat.")
	}
	if w != nil && w.TMax != nil && w.TMin != nil && *w.TMax-*w.TMin >= 10 {
		out.Tips = append(out.Tips, fmt.Sprintf("From %.0f°C to %.0f°C: wear layers you can take off.", *w.TMin, *w.TMax))
	}
	if w != nil && w.UV != nil && *w.UV >= 6 {
		out.Tips = append(out.Tips, "Strong sun: sunglasses and a hat.")
	}
	if out.Training != nil && !out.Training.Done {
		tm := ""
		if out.Training.Time != "" {
			tm = " at " + out.Training.Time
		}
		out.Tips = append(out.Tips, "Training"+tm+" ("+out.Training.Name+"): pack your gym clothes and trainers.")
	}
	if out.Outing != nil {
		out.Tips = append(out.Tips, "Outing: "+out.Outing.Title+" — layers, and shoes for the trail.")
	}
	for _, e := range out.Events {
		if e.Occasion == "wedding" || e.Occasion == "formal" || e.Occasion == "interview" {
			out.Tips = append(out.Tips, "“"+e.Title+"” — dress smart; the suggestions aim for it.")
			break
		}
	}
	return out
}

func (s *Server) handleToday(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	q := r.URL.Query()
	date := firstNonEmpty(q.Get("date"), today())
	if !validDate(date) {
		writeErr(w, http.StatusBadRequest, "the day is YYYY-MM-DD")
		return
	}
	shuffle := int64(queryInt(r, "shuffle"))
	out := map[string]any{"day": s.planDay(ctx, uid, date, q.Get("occasion"), q.Get("style"), shuffle)}
	if date == today() {
		tomorrow := store.AddDays(date, 1)
		c := s.clients(ctx, uid)
		tw, _ := s.homeWeather(ctx, c, tomorrow)
		tp, _ := s.store.PlanFor(ctx, uid, tomorrow)
		if tp != nil {
			s.decorateWear(tp)
		}
		out["tomorrow"] = map[string]any{"date": tomorrow, "weather": tw, "plan": tp}
		laundry, _ := s.store.ListItems(ctx, uid, store.ItemFilter{Status: "laundry"})
		out["laundry"] = len(laundry)
		out["nudge"] = s.nudge(ctx, uid)
	}
	writeJSON(w, http.StatusOK, out)
}

// nudge is one piece that has been waiting a long time to be worn.
func (s *Server) nudge(ctx context.Context, uid int64) *store.Item {
	list, _ := s.store.ListItems(ctx, uid, store.ItemFilter{Status: "ready"})
	var best *store.Item
	bestDays := 60
	for _, it := range list {
		if it.Category == "accessory" || it.Category == "other" || it.Category == "bag" {
			continue
		}
		since := it.LastWorn
		if since == "" {
			since = firstNonEmpty(it.BoughtOn, day10(it.CreatedAt))
		}
		if d := daysBetween(since, today()); d > bestDays {
			best, bestDays = it, d
		}
	}
	if best != nil {
		decorateItem(best)
	}
	return best
}

// handleSummary is the sidebar's numbers.
func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	all, _ := s.store.ListItems(ctx, uid, store.ItemFilter{})
	states := map[string]int{}
	for _, it := range all {
		states[it.Status]++
	}
	outfits, _ := s.store.ListOutfits(ctx, uid)
	plan, _ := s.store.PlanFor(ctx, uid, store.AddDays(today(), 1))
	worn, _ := s.store.ListWears(ctx, uid, today(), today())
	wornToday := false
	for _, x := range worn {
		if !x.Planned {
			wornToday = true
		}
	}
	packs, _ := s.store.ListPacks(ctx, uid)
	var next map[string]any
	for _, p := range packs {
		if p.StartDate >= today() || (p.EndDate >= today() && p.StartDate != "") {
			packed := 0
			for _, it := range p.Items {
				if it.Packed {
					packed++
				}
			}
			next = map[string]any{"id": p.ID, "name": p.Name, "start_date": p.StartDate, "days": daysBetween(today(), p.StartDate), "packed": packed, "total": len(p.Items)}
			break
		}
	}
	ideas, _ := s.store.ListIdeas(ctx, uid, "idea")
	writeJSON(w, http.StatusOK, map[string]any{"items": len(all), "outfits": len(outfits), "laundry": states["laundry"], "repair": states["repair"], "lent": states["lent"],
		"planned_tomorrow": plan != nil, "worn_today": wornToday, "next_trip": next, "ideas": len(ideas), "today": today()})
}

// handleStats is the wardrobe in numbers for a period.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	days := queryInt(r, "days")
	if days <= 0 || days > 3650 {
		days = 365
	}
	from := store.AddDays(today(), -days+1)
	all, _ := s.store.ListItems(ctx, uid, store.ItemFilter{})
	gone, _ := s.store.ListItems(ctx, uid, store.ItemFilter{Gone: true})
	counts, _ := s.store.WearCounts(ctx, uid, from)
	type row struct {
		Item  *store.Item `json:"item"`
		Wears int         `json:"wears"`
		Days  int         `json:"days_since"`
	}
	rowsAll := []row{}
	cats := map[string]int{}
	colors := map[string]int{}
	var value, spentYear int64
	worn := 0
	year := today()[:4]
	for _, it := range all {
		decorateItem(it)
		cats[it.Category]++
		if len(it.Colors) > 0 {
			colors[it.Colors[0].Name]++
		}
		if it.Price != nil {
			value += *it.Price
			if strings.HasPrefix(it.BoughtOn, year) {
				spentYear += *it.Price
			}
		}
		n := counts[it.ID]
		if n > 0 {
			worn++
		}
		since := -1
		if it.LastWorn != "" {
			since = daysBetween(it.LastWorn, today())
		}
		rowsAll = append(rowsAll, row{it, n, since})
	}
	most := append([]row(nil), rowsAll...)
	sort.SliceStable(most, func(i, j int) bool { return most[i].Wears > most[j].Wears })
	if len(most) > 8 {
		most = most[:8]
	}
	never := []row{}
	for _, x := range rowsAll {
		if x.Wears == 0 && x.Item.Category != "accessory" && x.Item.Category != "other" && daysBetween(firstNonEmpty(x.Item.BoughtOn, day10(x.Item.CreatedAt)), today()) > 30 {
			never = append(never, x)
		}
	}
	sort.SliceStable(never, func(i, j int) bool { return never[i].Days > never[j].Days })
	if len(never) > 12 {
		never = never[:12]
	}
	// cost per wear: the best value and the worst
	cpw := []*store.Item{}
	for _, it := range all {
		if it.Price != nil && it.Wears > 0 {
			cpw = append(cpw, it)
		}
	}
	sort.SliceStable(cpw, func(i, j int) bool { return *cpw[i].PerWear < *cpw[j].PerWear })
	bestValue, worstValue := cpw, []*store.Item{}
	if len(cpw) > 5 {
		bestValue = cpw[:5]
		worstValue = append([]*store.Item(nil), cpw[len(cpw)-5:]...)
		sort.SliceStable(worstValue, func(i, j int) bool { return *worstValue[i].PerWear > *worstValue[j].PerWear })
	}
	type kv struct {
		Key string `json:"key"`
		N   int    `json:"n"`
		Hex string `json:"hex,omitempty"`
	}
	palette := []kv{}
	for k, n := range colors {
		palette = append(palette, kv{k, n, style.HexOf(k)})
	}
	sort.SliceStable(palette, func(i, j int) bool { return palette[i].N > palette[j].N })
	categories := []kv{}
	for _, c := range style.Categories {
		if cats[c.ID] > 0 {
			categories = append(categories, kv{c.Plural, cats[c.ID], ""})
		}
	}
	// wears a week, for the chart
	wears, _ := s.store.ListWears(ctx, uid, from, today())
	weeks := map[string]int{}
	for _, x := range wears {
		if !x.Planned {
			t, _ := store.ParseDate(x.Date)
			wd := (int(t.Weekday()) + 6) % 7
			weeks[t.AddDate(0, 0, -wd).Format("2006-01-02")]++
		}
	}
	type wk struct {
		Week string `json:"week"`
		N    int    `json:"n"`
	}
	weekly := []wk{}
	t0, _ := store.ParseDate(from)
	t0 = t0.AddDate(0, 0, -((int(t0.Weekday()) + 6) % 7))
	for t := t0; !t.After(time.Now()); t = t.AddDate(0, 0, 7) {
		k := t.Format("2006-01-02")
		weekly = append(weekly, wk{k, weeks[k]})
	}
	if len(weekly) > 60 {
		weekly = weekly[len(weekly)-60:]
	}
	use := 0
	if len(all) > 0 {
		use = worn * 100 / len(all)
	}
	var soldFor int64
	donated := 0
	for _, g := range gone {
		if g.SoldFor != nil {
			soldFor += *g.SoldFor
		}
		if g.Gone == "donated" {
			donated++
		}
	}
	st := s.settingsOf(ctx, uid)
	writeJSON(w, http.StatusOK, map[string]any{"days": days, "pieces": len(all), "worn_pieces": worn, "use_pct": use, "days_logged": s.store.DaysWorn(ctx, uid, from),
		"value": value, "spent_year": spentYear, "currency": st.Currency, "most_worn": most, "never_worn": never, "best_value": bestValue, "worst_value": worstValue,
		"palette": palette, "categories": categories, "weekly": weekly, "gone": len(gone), "donated": donated, "sold_for": soldFor})
}

// handleDay is a day for Pocket Day and Pocket AI: what was worn (or is planned).
func (s *Server) handleDay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	date := chiParam(r, "date")
	if !validDate(date) {
		writeErr(w, http.StatusBadRequest, "the day is YYYY-MM-DD")
		return
	}
	list, _ := s.store.ListWears(ctx, uid, date, date)
	out := []map[string]any{}
	for _, x := range list {
		items, _ := s.store.ItemsByIDs(ctx, uid, x.ItemIDs)
		names := []string{}
		for _, id := range x.ItemIDs {
			if it := items[id]; it != nil {
				names = append(names, it.Name)
			}
		}
		out = append(out, map[string]any{"id": x.ID, "planned": x.Planned, "items": names, "occasion": x.Occasion, "event": x.Event, "notes": x.Notes, "feel": x.Feel,
			"image": s.shareURL(uid, x.ItemIDs), "url": s.link("/plan?date=" + date)})
	}
	summary := ""
	for _, o := range out {
		if o["planned"] == false {
			summary = "Wore " + strings.Join(o["items"].([]string), ", ")
		} else if summary == "" {
			summary = "Planned: " + strings.Join(o["items"].([]string), ", ")
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": date, "wears": out, "summary": summary})
}

// handleSearch finds pieces and outfits by name, brand, colour or notes.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"items": []any{}, "outfits": []any{}, "results": []any{}})
		return
	}
	items, _ := s.store.ListItems(ctx, uid, store.ItemFilter{Q: q})
	if len(items) > 30 {
		items = items[:30]
	}
	for _, it := range items {
		decorateItem(it)
	}
	ql := strings.ToLower(q)
	outfits, _ := s.store.ListOutfits(ctx, uid)
	found := []*store.Outfit{}
	for _, o := range outfits {
		if strings.Contains(strings.ToLower(o.Name+" "+o.Notes+" "+o.Occasion), ql) && len(found) < 20 {
			found = append(found, s.decorateOutfit(o))
		}
	}
	results := []map[string]any{}
	for _, it := range items {
		results = append(results, map[string]any{"id": it.ID, "title": it.Name, "subtitle": joinNonEmpty(" · ", it.Brand, it.Category), "type": "item", "path": "/closet/" + itoa(it.ID), "url": s.link("/closet/" + itoa(it.ID))})
	}
	for _, o := range found {
		results = append(results, map[string]any{"id": o.ID, "title": o.Name, "subtitle": o.Occasion, "type": "outfit", "path": "/outfits/" + itoa(o.ID), "url": s.link("/outfits/" + itoa(o.ID))})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "outfits": found, "results": results})
}
