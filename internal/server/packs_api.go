package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/style"
)

// ---- packing for a trip ----

type packInput struct {
	Name        *string            `json:"name"`
	Destination *string            `json:"destination"`
	Lat         *float64           `json:"lat"`
	Lng         *float64           `json:"lng"`
	StartDate   *string            `json:"start_date"`
	EndDate     *string            `json:"end_date"`
	Purpose     *string            `json:"purpose"`
	Items       *[]store.PackItem  `json:"items"`
	Extras      *[]store.PackExtra `json:"extras"`
	Notes       *string            `json:"notes"`
	TripRef     *string            `json:"trip_ref"`
	TripURL     *string            `json:"trip_url"`
}

func (s *Server) applyPack(r *http.Request, in packInput, p *store.Pack) string {
	ctx := r.Context()
	t := func(v *string, n int) string { return clip(strings.TrimSpace(*v), n) }
	if in.Name != nil {
		p.Name = t(in.Name, 120)
	}
	if in.Destination != nil {
		p.Destination = t(in.Destination, 160)
	}
	if in.Lat != nil && in.Lng != nil {
		if *in.Lat < -90 || *in.Lat > 90 || *in.Lng < -180 || *in.Lng > 180 {
			return "that place is off the map"
		}
		p.Lat, p.Lng = in.Lat, in.Lng
	}
	if in.StartDate != nil {
		if !validDate(*in.StartDate) {
			return "the first day is YYYY-MM-DD"
		}
		p.StartDate = *in.StartDate
	}
	if in.EndDate != nil {
		if !validDate(*in.EndDate) {
			return "the last day is YYYY-MM-DD"
		}
		p.EndDate = *in.EndDate
	}
	if p.StartDate != "" && p.EndDate != "" && p.EndDate < p.StartDate {
		return "the trip ends before it starts"
	}
	if in.Purpose != nil {
		if !oneOf(*in.Purpose, store.Purposes) {
			return "unknown kind of trip"
		}
		p.Purpose = *in.Purpose
	}
	if in.Items != nil {
		want := []int64{}
		for _, it := range *in.Items {
			want = append(want, it.ItemID)
		}
		own, _ := s.store.OwnIDs(ctx, actorID(ctx), want)
		list := []store.PackItem{}
		for _, it := range *in.Items {
			if hasInt(own, it.ItemID) && len(list) < 120 {
				if it.Qty <= 0 {
					it.Qty = 1
				}
				list = append(list, it)
			}
		}
		p.Items = list
	}
	if in.Extras != nil {
		list := []store.PackExtra{}
		for _, e := range *in.Extras {
			if e.Text = clip(strings.TrimSpace(e.Text), 120); e.Text != "" && len(list) < 80 {
				list = append(list, e)
			}
		}
		p.Extras = list
	}
	if in.Notes != nil {
		p.Notes = t(in.Notes, 3000)
	}
	if in.TripRef != nil {
		p.TripRef = t(in.TripRef, 60)
	}
	if in.TripURL != nil {
		p.TripURL = t(in.TripURL, 400)
	}
	if p.Name == "" {
		p.Name = firstNonEmpty(p.Destination, "Trip")
	}
	return ""
}

func (s *Server) handleListPacks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	list, err := s.store.ListPacks(ctx, uid)
	if err != nil {
		s.fail(w, err, "packing")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"packs": list})
}

func (s *Server) handleGetPack(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPack(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "packing")
		return
	}
	ids := []int64{}
	for _, it := range p.Items {
		ids = append(ids, it.ItemID)
	}
	items, _ := s.store.ItemsByIDs(ctx, uid, ids)
	for _, it := range items {
		decorateItem(it)
	}
	pieces, _ := s.pieces(ctx, uid, today())
	var ps []style.Piece
	for _, x := range pieces {
		if hasInt(ids, x.ID) {
			x.Ready = true
			ps = append(ps, x)
		}
	}
	c := s.packContext(r, p)
	n, ex := style.Count(ps, c, 62, 0, 4)
	writeJSON(w, http.StatusOK, map[string]any{"pack": p, "items": items, "outfits": n, "examples": ex})
}

// packContext is the weather and kind of trip for the engine.
func (s *Server) packContext(r *http.Request, p *store.Pack) style.Context {
	ctx := r.Context()
	c := s.baseContext(ctx, actorID(ctx), "")
	c.Season = ""
	if t, err := store.ParseDate(p.StartDate); err == nil {
		c.Season = style.SeasonOf(t)
	}
	var w dayWeather
	if len(p.Weather) > 0 && json.Unmarshal(p.Weather, &w) == nil {
		w.applyTo(&c)
	}
	return c
}

func (s *Server) handleCreatePack(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in packInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	p := &store.Pack{UserID: uid, Purpose: "holiday", Items: []store.PackItem{}, Extras: []store.PackExtra{}}
	if msg := s.applyPack(r, in, p); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	s.packWeather(r, p)
	if err := s.store.CreatePack(ctx, p); err != nil {
		s.fail(w, err, "packing")
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// packWeather keeps the forecast for the trip's first days (when it is near enough).
func (s *Server) packWeather(r *http.Request, p *store.Pack) {
	ctx := r.Context()
	if p.Lat == nil || p.Lng == nil || p.StartDate == "" {
		return
	}
	d := daysBetween(today(), p.StartDate)
	if d < 0 || d > forecastDays {
		return
	}
	c := s.clients(ctx, actorID(ctx))
	var hi, lo *float64
	rain := false
	var first *dayWeather
	for i := 0; i <= daysBetween(p.StartDate, firstNonEmpty(p.EndDate, p.StartDate)) && d+i <= forecastDays && i < 7; i++ {
		wx, _, err := s.forecastFor(ctx, c, *p.Lat, *p.Lng, store.AddDays(p.StartDate, i))
		if err != nil || wx == nil {
			break
		}
		if first == nil {
			first = wx
		}
		if wx.TMax != nil && (hi == nil || *wx.TMax > *hi) {
			hi = wx.TMax
		}
		if wx.TMin != nil && (lo == nil || *wx.TMin < *lo) {
			lo = wx.TMin
		}
		rain = rain || wx.Rainy()
	}
	if first == nil {
		return
	}
	first.TMax, first.TMin, first.Hours = hi, lo, nil
	if rain && !first.Rainy() {
		v := 60.0
		first.PrecipProb = &v
	}
	b, _ := json.Marshal(first)
	p.Weather = b
}

func (s *Server) handleUpdatePack(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPack(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "packing")
		return
	}
	var in packInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if msg := s.applyPack(r, in, p); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if in.StartDate != nil || in.Lat != nil || queryBool(r, "weather") {
		s.packWeather(r, p)
	}
	if err := s.store.UpdatePack(ctx, p); err != nil {
		s.fail(w, err, "packing")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleDeletePack(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeletePack(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "packing")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// handlePackFill picks a small set of pieces that make many outfits for
// the trip's days, weather and kind (a capsule), and the extras to bring.
func (s *Server) handlePackFill(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPack(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "packing")
		return
	}
	days := 3
	if p.StartDate != "" && p.EndDate != "" {
		days = daysBetween(p.StartDate, p.EndDate) + 1
	}
	pieces, items := s.pieces(ctx, uid, today())
	for i := range pieces {
		// the laundry will be done by then; lent and at the tailor will not
		if it := items[pieces[i].ID]; it != nil {
			pieces[i].Ready = it.Status != "lent" && it.Status != "repair"
		}
	}
	c := s.packContext(r, p)
	res := style.Capsule(pieces, days, c, p.Purpose)
	have := map[int64]bool{}
	for _, it := range p.Items {
		have[it.ItemID] = true
	}
	for _, id := range res.Items {
		if !have[id] {
			p.Items = append(p.Items, store.PackItem{ItemID: id, Qty: 1})
		}
	}
	haveX := map[string]bool{}
	for _, e := range p.Extras {
		haveX[strings.ToLower(e.Text)] = true
	}
	for _, e := range res.Extras {
		if !haveX[strings.ToLower(e)] {
			p.Extras = append(p.Extras, store.PackExtra{Text: e})
		}
	}
	if err := s.store.UpdatePack(ctx, p); err != nil {
		s.fail(w, err, "packing")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pack": p, "outfits": res.Outfits, "notes": res.Notes, "text": fmt.Sprintf("%d pieces, about %d outfits", len(res.Items), res.Outfits)})
}

// handleTrips lists the person's coming trips in Pocket Travel (to pack for one).
func (s *Server) handleTrips(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	list, err := s.trips(ctx, s.clients(ctx, actorID(ctx)))
	if err != nil {
		s.userFail(w, err, "trips")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"trips": list})
}

// handlePackNote saves a packing list in Pocket Notes as a checklist.
func (s *Server) handlePackNote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPack(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "packing")
		return
	}
	ids := []int64{}
	for _, it := range p.Items {
		ids = append(ids, it.ItemID)
	}
	items, _ := s.store.ItemsByIDs(ctx, uid, ids)
	var b strings.Builder
	fmt.Fprintf(&b, "Packing for %s", p.Name)
	if p.StartDate != "" {
		fmt.Fprintf(&b, " (%s – %s)", niceDate(p.StartDate), niceDate(firstNonEmpty(p.EndDate, p.StartDate)))
	}
	b.WriteString("\n\n")
	for _, it := range p.Items {
		if x := items[it.ItemID]; x != nil {
			mark := " "
			if it.Packed {
				mark = "x"
			}
			fmt.Fprintf(&b, "- [%s] %s\n", mark, x.Name)
		}
	}
	for _, e := range p.Extras {
		mark := " "
		if e.Packed {
			mark = "x"
		}
		fmt.Fprintf(&b, "- [%s] %s\n", mark, e.Text)
	}
	u, err := s.toNotes(ctx, s.clients(ctx, uid), "Packing: "+p.Name, b.String())
	if err != nil {
		s.userFail(w, err, "notes")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": u, "text": "Saved in Pocket Notes"})
}
