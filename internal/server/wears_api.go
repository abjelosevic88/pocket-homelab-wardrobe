package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/style"
)

// ---- what was worn, what is planned ----

type wearInput struct {
	Date     *string  `json:"date"`
	Planned  *bool    `json:"planned"`
	OutfitID *int64   `json:"outfit_id"`
	ItemIDs  *[]int64 `json:"item_ids"`
	Occasion *string  `json:"occasion"`
	Event    *string  `json:"event"`
	EventRef *string  `json:"event_ref"`
	Feel     *int     `json:"feel"`
	Notes    *string  `json:"notes"`
	PhotoID  *int64   `json:"photo_id"`
}

func (s *Server) applyWear(r *http.Request, in wearInput, w *store.Wear) string {
	ctx := r.Context()
	uid := actorID(ctx)
	if in.Date != nil {
		if *in.Date == "" || !validDate(*in.Date) {
			return "the day is YYYY-MM-DD"
		}
		w.Date = *in.Date
	}
	if in.Planned != nil {
		w.Planned = *in.Planned
	}
	if in.OutfitID != nil {
		if *in.OutfitID == 0 {
			w.OutfitID = nil
		} else {
			o, err := s.store.GetOutfit(ctx, uid, *in.OutfitID)
			if err != nil {
				return "unknown outfit"
			}
			w.OutfitID = &o.ID
			if in.ItemIDs == nil {
				w.ItemIDs = o.ItemIDs
			}
		}
	}
	if in.ItemIDs != nil {
		own, err := s.store.OwnIDs(ctx, uid, *in.ItemIDs)
		if err != nil {
			return "unknown pieces"
		}
		w.ItemIDs = own
	}
	if in.Occasion != nil {
		if *in.Occasion != "" && style.OccasionByID(*in.Occasion).ID != *in.Occasion {
			return "unknown occasion"
		}
		w.Occasion = *in.Occasion
	}
	if in.Event != nil {
		w.Event = clip(strings.TrimSpace(*in.Event), 200)
	}
	if in.EventRef != nil {
		w.EventRef = clip(strings.TrimSpace(*in.EventRef), 120)
	}
	if in.Feel != nil {
		if *in.Feel < 0 || *in.Feel > 5 {
			return "how it felt is 0 to 5"
		}
		w.Feel = *in.Feel
	}
	if in.Notes != nil {
		w.Notes = clip(strings.TrimSpace(*in.Notes), 2000)
	}
	if in.PhotoID != nil {
		id, ok := s.photoMine(ctx, uid, in.PhotoID)
		if !ok {
			return "unknown photo"
		}
		w.PhotoID = id
	}
	if len(w.ItemIDs) == 0 {
		return "pick what you wore"
	}
	return ""
}

func (s *Server) decorateWear(w *store.Wear) *store.Wear {
	w.PhotoURL, _ = photoURL(w.PhotoID)
	return w
}

func (s *Server) handleListWears(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	q := r.URL.Query()
	from, to := firstNonEmpty(q.Get("from"), store.AddDays(today(), -30)), firstNonEmpty(q.Get("to"), store.AddDays(today(), 14))
	if !validDate(from) || !validDate(to) || daysBetween(from, to) > 400 {
		writeErr(w, http.StatusBadRequest, "from and to are YYYY-MM-DD, at most a year apart")
		return
	}
	list, err := s.store.ListWears(ctx, uid, from, to)
	if err != nil {
		s.fail(w, err, "wears")
		return
	}
	ids := []int64{}
	for _, x := range list {
		s.decorateWear(x)
		ids = append(ids, x.ItemIDs...)
	}
	items, _ := s.store.ItemsByIDs(ctx, uid, ids)
	for _, it := range items {
		decorateItem(it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"wears": list, "items": items, "from": from, "to": to})
}

// handleCreateWear logs what was worn (or plans a day). Worn today: the
// weather is kept with it, and tops go to the laundry when that is on.
func (s *Server) handleCreateWear(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in wearInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	x := &store.Wear{UserID: uid, Date: today(), ItemIDs: []int64{}, Source: linkApp(ctx)}
	if msg := s.applyWear(r, in, x); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if x.Planned && x.Date < today() {
		writeErr(w, http.StatusBadRequest, "a plan is for today or a day ahead")
		return
	}
	if !x.Planned && x.Date > today() {
		writeErr(w, http.StatusBadRequest, "that day has not come yet: plan it instead")
		return
	}
	c := s.clients(ctx, uid)
	if wx, _ := s.homeWeather(ctx, c, x.Date); wx != nil {
		b, _ := json.Marshal(map[string]any{"t_max": wx.TMax, "t_min": wx.TMin, "condition": wx.Condition, "code": wx.Code, "place": wx.Place})
		x.Weather = b
	}
	if x.Planned {
		if old, err := s.store.PlanFor(ctx, uid, x.Date); err == nil {
			_ = s.store.DeleteWear(ctx, uid, old.ID)
		}
	}
	if x.OutfitID == nil {
		if o, err := s.store.FindOutfit(ctx, uid, x.ItemIDs); err == nil {
			x.OutfitID = &o.ID
		}
	}
	if err := s.store.CreateWear(ctx, x); err != nil {
		s.fail(w, err, "wear")
		return
	}
	s.afterWorn(r, x, c.st)
	writeJSON(w, http.StatusCreated, s.decorateWear(x))
}

// afterWorn: the laundry, and the event for the rest of the box.
func (s *Server) afterWorn(r *http.Request, x *store.Wear, st store.Settings) {
	ctx := r.Context()
	uid := actorID(ctx)
	items, _ := s.store.ItemsByIDs(ctx, uid, x.ItemIDs)
	names := []string{}
	for _, id := range x.ItemIDs {
		if it := items[id]; it != nil {
			names = append(names, it.Name)
		}
	}
	if x.Planned {
		s.publish(ctx, uid, "wardrobe.outfit_planned", "wear:"+itoa(x.ID), map[string]any{"id": x.ID, "date": x.Date, "items": names, "occasion": x.Occasion, "url": s.link("/plan")})
		return
	}
	if st.AutoLaundry && x.Date >= store.AddDays(today(), -1) {
		var wash []int64
		for _, id := range x.ItemIDs {
			if it := items[id]; it != nil && (it.Category == "top" || it.Category == "active") && it.Status == "ready" {
				wash = append(wash, id)
			}
		}
		if len(wash) > 0 {
			_, _ = s.store.SetItemStatus(ctx, uid, wash, "laundry", "")
		}
		s.laundryCheck(ctx, uid, st)
	}
	s.publish(ctx, uid, "wardrobe.outfit_worn", "wear:"+itoa(x.ID), map[string]any{"id": x.ID, "date": x.Date, "items": names, "occasion": x.Occasion, "url": s.link("/plan")})
}

func (s *Server) ownWear(w http.ResponseWriter, r *http.Request, write bool) *store.Wear {
	ctx := r.Context()
	x, err := s.store.GetWear(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "wear")
		return nil
	}
	if app := linkApp(ctx); write && app != "" && x.Source != app {
		writeErr(w, http.StatusNotFound, "not found")
		return nil
	}
	return x
}

func (s *Server) handleUpdateWear(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	x := s.ownWear(w, r, true)
	if x == nil {
		return
	}
	var in wearInput
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	wasPlanned := x.Planned
	if msg := s.applyWear(r, in, x); msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	if !x.Planned && x.Date > today() {
		writeErr(w, http.StatusBadRequest, "that day has not come yet")
		return
	}
	if err := s.store.UpdateWear(ctx, x); err != nil {
		s.fail(w, err, "wear")
		return
	}
	if wasPlanned && !x.Planned {
		s.afterWorn(r, x, s.settingsOf(ctx, actorID(ctx)))
	}
	writeJSON(w, http.StatusOK, s.decorateWear(x))
}

func (s *Server) handleDeleteWear(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	x := s.ownWear(w, r, true)
	if x == nil {
		return
	}
	if err := s.store.DeleteWear(ctx, actorID(ctx), x.ID); err != nil {
		s.fail(w, err, "wear")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// laundryCheck tells the person (once a day) when the basket is full.
func (s *Server) laundryCheck(ctx context.Context, uid int64, st store.Settings) {
	if st.LaundryAlert <= 0 {
		return
	}
	list, err := s.store.ListItems(ctx, uid, store.ItemFilter{Status: "laundry"})
	if err != nil || len(list) < st.LaundryAlert {
		return
	}
	if !s.once(ctx, "laundry:"+itoa(uid)+":"+today()) {
		return
	}
	s.publish(ctx, uid, "wardrobe.laundry_full", "laundry:"+today(), map[string]any{"count": len(list), "url": s.link("/laundry")})
	s.push(ctx, uid, st, "Laundry day", itoa(int64(len(list)))+" pieces are waiting to be washed.", s.link("/laundry"))
}
