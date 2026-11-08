package server

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/style"
)

var (
	reEmail = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	reTime  = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
	reCur   = regexp.MustCompile(`^[A-Z]{3}$`)
)

// userErr is a reason the person can act on (400, not 502).
type userErr struct{ msg string }

func (e *userErr) Error() string { return e.msg }

// today is the local day (the box's zone).
func today() string { return time.Now().Format("2006-01-02") }

func (s *Server) settingsOf(ctx context.Context, uid int64) store.Settings {
	st, _ := s.store.GetSettings(ctx, uid)
	return st
}

func (s *Server) userFail(w http.ResponseWriter, err error, what string) {
	var ue *userErr
	if errors.As(err, &ue) {
		writeErr(w, http.StatusBadRequest, ue.msg)
		return
	}
	if isNotConnected(err) {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.fail(w, err, what)
}

// link is a page of Wardrobe as a full address (for messages and events).
func (s *Server) link(path string) string {
	return strings.TrimRight(s.cfg.BaseURL, "/") + path
}

func niceDate(d string) string {
	t, err := store.ParseDate(d)
	if err != nil {
		return d
	}
	return t.Format("Mon 2 Jan")
}

func whenWords(d int) string {
	switch {
	case d < -1:
		return fmt.Sprintf("%d days ago", -d)
	case d == -1:
		return "yesterday"
	case d == 0:
		return "today"
	case d == 1:
		return "tomorrow"
	}
	return fmt.Sprintf("in %d days", d)
}

func daysBetween(a, b string) int {
	ta, e1 := store.ParseDate(a)
	tb, e2 := store.ParseDate(b)
	if e1 != nil || e2 != nil {
		return 0
	}
	return int(math.Round(tb.Sub(ta).Hours() / 24))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// ---- the wardrobe as the style engine sees it ----

// pieces is everything the person has (not given away), for the engine,
// as on a day (how long since each was worn).
func (s *Server) pieces(ctx context.Context, uid int64, day string) ([]style.Piece, map[int64]*store.Item) {
	list, _ := s.store.ListItems(ctx, uid, store.ItemFilter{})
	last, _ := s.store.LastWorn(ctx, uid, day)
	out := make([]style.Piece, 0, len(list))
	byID := map[int64]*store.Item{}
	for _, it := range list {
		byID[it.ID] = it
		out = append(out, pieceOf(it, last[it.ID], day))
	}
	return out, byID
}

func pieceOf(it *store.Item, last, day string) style.Piece {
	p := style.Piece{ID: it.ID, Name: it.Name, Category: it.Category, Kind: it.Kind, Pattern: it.Pattern, Warmth: it.Warmth, Formality: it.Formality,
		Styles: it.Styles, Seasons: it.Seasons, Rain: it.Rain, Favourite: it.Favourite, DaysSince: -1, Wears: it.Wears, Ready: it.Status == "ready"}
	for _, c := range it.Colors {
		p.Colors = append(p.Colors, c.Hex)
	}
	if last != "" {
		p.DaysSince = daysBetween(last, day)
	}
	return p
}

// baseContext is the person's own taste for the engine, without the weather.
func (s *Server) baseContext(ctx context.Context, uid int64, styleID string) style.Context {
	st := s.settingsOf(ctx, uid)
	c := style.Context{Styles: st.Styles, Avoid: st.AvoidColors, Love: st.LoveColors, Season: style.SeasonOf(time.Now()), Occasion: "everyday"}
	c.Style = styleID
	if c.Style == "" && len(st.Styles) > 0 {
		c.Style = st.Styles[0]
	}
	if c.Style == "any" {
		c.Style = ""
	}
	return c
}

// dayContext adds a day's weather (where the person is) to their taste.
func (s *Server) dayContext(ctx context.Context, c *clients, date, occasion, styleID string) (style.Context, *dayWeather, string) {
	sc := s.baseContext(ctx, c.u.ID, styleID)
	if t, err := store.ParseDate(date); err == nil {
		sc.Season = style.SeasonOf(t)
	}
	if occasion != "" {
		sc.Occasion = occasion
	}
	sc.Indoor = oneOf(sc.Occasion, []string{"work", "formal_work", "interview", "dinner", "party", "wedding", "formal", "date"})
	w, why := s.homeWeather(ctx, c, date)
	w.applyTo(&sc)
	return sc, w, why
}
