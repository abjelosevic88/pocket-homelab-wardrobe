package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/notify"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
)

// pushes runs every minute; at each person's morning hour it suggests
// today's outfit (or reminds them of the one they planned), at their
// evening hour it gives tomorrow's weather and what is on, so they can
// plan.
func (s *Server) pushes(ctx context.Context) {
	now := time.Now()
	if now.Minute() > 10 {
		return
	}
	ids, err := s.store.SettingsUsers(ctx)
	if err != nil {
		return
	}
	for _, uid := range ids {
		st, err := s.store.GetSettings(ctx, uid)
		if err != nil || st.NotifyURL == "" {
			continue
		}
		if st.NotifyMorning && now.Hour() == st.NotifyHour && s.once(ctx, fmt.Sprintf("morning:%d:%s", uid, today())) {
			s.dayMessage(ctx, uid, st, today(), true)
		}
		if st.NotifyEvening && now.Hour() == st.EveningHour && s.once(ctx, fmt.Sprintf("evening:%d:%s", uid, today())) {
			s.dayMessage(ctx, uid, st, store.AddDays(today(), 1), true)
		}
	}
}

// dayMessage builds the message about one day (and sends it when asked).
func (s *Server) dayMessage(ctx context.Context, uid int64, st store.Settings, date string, send bool) string {
	d := s.planDay(ctx, uid, date, "", "", 0)
	when := "Today"
	if date != today() {
		when = "Tomorrow"
	}
	var b strings.Builder
	if d.Weather != nil {
		fmt.Fprintf(&b, "%s: %s.\n", when, d.Weather.short())
	}
	for i, e := range d.Events {
		if i >= 3 {
			break
		}
		fmt.Fprintf(&b, "• %s %s\n", firstNonEmpty(e.Time, "all day"), e.Title)
	}
	names := func(ids []int64) string {
		var n []string
		for _, id := range ids {
			if it := d.Items[id]; it != nil {
				n = append(n, it.Name)
			}
		}
		return strings.Join(n, ", ")
	}
	switch {
	case d.Plan != nil:
		b.WriteString("Planned: " + names(d.Plan.ItemIDs) + ".\n")
	case len(d.Suggest.Suggestions) > 0:
		b.WriteString("Try: " + names(d.Suggest.Suggestions[0].Items) + ".\n")
	}
	for i, t := range d.Tips {
		if i < 2 {
			b.WriteString(t + "\n")
		}
	}
	body := strings.TrimSpace(b.String())
	if body == "" {
		return ""
	}
	if send {
		title := "What to wear today"
		if date != today() {
			title = "Tomorrow"
		}
		s.push(ctx, uid, st, title, body, s.link("/?date="+date))
	}
	return body
}

// once is true the first time it is asked for a key.
func (s *Server) once(ctx context.Context, key string) bool {
	if _, _, done := s.store.CacheGet(ctx, key); done {
		return false
	}
	_ = s.store.CachePut(ctx, key, "1")
	return true
}

// push sends one message to a person's own notification address.
func (s *Server) push(ctx context.Context, uid int64, st store.Settings, title, body, link string) bool {
	if st.NotifyURL == "" {
		return false
	}
	n := notify.New(st.NotifyURL, st.NotifyFormat)
	if err := n.Send(ctx, notify.Message{Title: title, Body: body, Priority: "info", Tags: []string{"dress"}, Link: link}); err != nil {
		s.log.Warn("notify failed", "user", uid, "err", err)
		return false
	}
	return true
}

func (s *Server) handleNotifyTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st, _ := s.store.GetSettings(ctx, actorID(ctx))
	n := notify.New(st.NotifyURL, st.NotifyFormat)
	if err := n.Send(ctx, notify.Message{Title: "Pocket Wardrobe", Body: notifyTestBody, Priority: "info", Tags: []string{"dress"}}); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleCheckNow builds tomorrow's (or today's) message now, and sends it when asked.
func (s *Server) handleCheckNow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	st := s.settingsOf(ctx, uid)
	date := store.AddDays(today(), 1)
	if r.URL.Query().Get("day") == "today" {
		date = today()
	}
	body := s.dayMessage(ctx, uid, st, date, queryBool(r, "send"))
	if body == "" {
		body = "Nothing to say about that day yet: add a few pieces."
	}
	writeJSON(w, http.StatusOK, map[string]any{"text": body})
}

const notifyTestBody = "Notifications work. In the morning you get an outfit for the day's weather and plans, in the evening tomorrow's weather, and a note when the laundry basket is full."
