package server

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/style"
)

// Wardrobe for the apps that bring the box together (capabilities in
// links.go): the same things /search, /day/{date}, /summary and /today read,
// in the one shape Pocket Search, Pocket Day and Pocket Hub ask every app
// for. Each runs as the request's person and reads only what is theirs.
// Addresses are paths in the app; the one who asks makes them whole.

// occasionLabel is an occasion in words ("" when none was said).
func occasionLabel(id string) string {
	if o := style.OccasionByID(id); id != "" && o.ID == id {
		return o.Label
	}
	return ""
}

// kindLabel is a kind of piece in words ("" when unknown).
func kindLabel(id string) string {
	k, _ := style.KindByID(id)
	return k.Label
}

// stateWords says where a piece is when it is not in the closet, ready to wear.
var stateWords = map[string]string{"laundry": "in the laundry", "repair": "at the tailor", "lent": "lent out", "stored": "stored away"}

// piecesLine is a few pieces as one short line: "White shirt, Navy chinos and 2 more".
func piecesLine(names []string) string {
	if len(names) > 4 {
		return strings.Join(names[:3], ", ") + fmt.Sprintf(" and %d more", len(names)-3)
	}
	return strings.Join(names, ", ")
}

// pieceNames are the names of a person's pieces, in the order given.
func (s *Server) pieceNames(ctx context.Context, uid int64, ids []int64) []string {
	items, _ := s.store.ItemsByIDs(ctx, uid, ids)
	names := []string{}
	for _, id := range ids {
		if it := items[id]; it != nil && it.Name != "" {
			names = append(names, it.Name)
		}
	}
	return names
}

// wearTitle is a worn or planned day in one line: the saved outfit's name,
// else its pieces.
func (s *Server) wearTitle(ctx context.Context, uid int64, x *store.Wear) (title, pieces string) {
	pieces = piecesLine(s.pieceNames(ctx, uid, x.ItemIDs))
	if x.OutfitID != nil {
		if o, err := s.store.GetOutfit(ctx, uid, *x.OutfitID); err == nil && o.Name != "" {
			return o.Name, pieces
		}
	}
	return firstNonEmpty(pieces, "Outfit"), ""
}

// pocketSearch finds pieces, saved outfits, packing lists and kept ideas by words.
func (s *Server) pocketSearch(r *http.Request, q string, limit int) ([]core.SearchItem, error) {
	ctx := r.Context()
	uid := actorID(ctx)
	ql := strings.ToLower(q)
	has := func(parts ...string) bool { return strings.Contains(strings.ToLower(strings.Join(parts, " ")), ql) }
	out := []core.SearchItem{}
	// words in the name count more than words in the notes
	add := func(it core.SearchItem) {
		switch t := strings.ToLower(it.Title); {
		case t == ql:
			it.Score = 3
		case strings.Contains(t, ql):
			it.Score = 2
		default:
			it.Score = 1
		}
		out = append(out, it)
	}

	items, err := s.store.ListItems(ctx, uid, store.ItemFilter{Q: q})
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		color := ""
		if len(it.Colors) > 0 {
			color = it.Colors[0].Name
		}
		add(core.SearchItem{ID: itoa(it.ID), Title: it.Name, Snippet: joinNonEmpty(" · ", kindLabel(it.Kind), color, it.Brand, stateWords[it.Status]),
			Kind: "item", Date: it.LastWorn, URL: "/closet/" + itoa(it.ID)})
	}

	outfits, err := s.store.ListOutfits(ctx, uid)
	if err != nil {
		return nil, err
	}
	found := 0
	for _, o := range outfits {
		// each match reads its pieces: no more of them than can be asked for
		if found == 50 || !has(o.Name, o.Notes, o.Occasion, occasionLabel(o.Occasion)) {
			continue
		}
		found++
		title, pieces := o.Name, piecesLine(s.pieceNames(ctx, uid, o.ItemIDs))
		if title == "" {
			title, pieces = firstNonEmpty(pieces, "Outfit"), ""
		}
		add(core.SearchItem{ID: itoa(o.ID), Title: title, Snippet: joinNonEmpty(" · ", occasionLabel(o.Occasion), pieces), Kind: "outfit", Date: o.LastWorn, URL: "/outfits/" + itoa(o.ID)})
	}

	packs, err := s.store.ListPacks(ctx, uid)
	if err != nil {
		return nil, err
	}
	for _, p := range packs {
		if !has(p.Name, p.Destination, p.Purpose, p.Notes) {
			continue
		}
		days := ""
		if p.StartDate != "" {
			days = niceDate(p.StartDate)
			if p.EndDate != "" && p.EndDate != p.StartDate {
				days += " to " + niceDate(p.EndDate)
			}
		}
		where := p.Destination
		if p.Name == "" || strings.EqualFold(p.Name, where) {
			where = ""
		}
		add(core.SearchItem{ID: itoa(p.ID), Title: firstNonEmpty(p.Name, p.Destination, "Packing list"), Snippet: joinNonEmpty(" · ", where, days, packedWords(p)),
			Kind: "packing list", Date: p.StartDate, URL: "/packing/" + itoa(p.ID)})
	}

	ideas, err := s.store.ListIdeas(ctx, uid, "")
	if err != nil {
		return nil, err
	}
	for _, i := range ideas {
		// the ones still kept: not bought, not hidden
		if (i.Status != "idea" && i.Status != "wish") || !has(i.Title, kindLabel(i.Kind), i.ColorName, i.Why) {
			continue
		}
		makes := ""
		if i.Unlocks > 0 {
			makes = fmt.Sprintf("makes %d new %s", i.Unlocks, plural(i.Unlocks, "outfit", "outfits"))
		}
		add(core.SearchItem{ID: itoa(i.ID), Title: i.Title, Snippet: joinNonEmpty(" · ", kindLabel(i.Kind), i.ColorName, makes, i.PriceHint),
			Kind: "idea", Date: day10(i.CreatedAt), URL: "/shop", Link: i.URL})
	}

	sort.SliceStable(out, func(a, b int) bool { return out[a].Score > out[b].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// packedWords says how far the packing is: "4 of 10 packed".
func packedWords(p *store.Pack) string {
	if len(p.Items) == 0 {
		return ""
	}
	packed := 0
	for _, it := range p.Items {
		if it.Packed {
			packed++
		}
	}
	return fmt.Sprintf("%d of %d packed", packed, len(p.Items))
}

// pocketDay is what was worn on a day and what is planned for it. A wear
// belongs to the day the person gave it, so the caller's time zone changes nothing.
func (s *Server) pocketDay(r *http.Request, date string, _ *time.Location) (*core.Day, error) {
	ctx := r.Context()
	uid := actorID(ctx)
	list, err := s.store.ListWears(ctx, uid, date, date)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	day := &core.Day{Date: date}
	worn, planned := 0, 0
	for _, x := range list {
		kind := "worn"
		if x.Planned {
			kind = "planned"
			planned++
		} else {
			worn++
		}
		title, pieces := s.wearTitle(ctx, uid, x)
		day.Items = append(day.Items, core.DayItem{ID: itoa(x.ID), Title: title, Sub: joinNonEmpty(" · ", occasionLabel(x.Occasion), x.Event, pieces), Kind: kind,
			URL: "/plan?date=" + date, Image: s.sharePath(uid, x.ItemIDs), Note: x.Notes})
	}
	var parts []string
	if worn > 0 {
		parts = append(parts, fmt.Sprintf("Wore %d %s", worn, plural(worn, "outfit", "outfits")))
	}
	if planned > 0 {
		parts = append(parts, fmt.Sprintf("%d %s planned", planned, plural(planned, "outfit", "outfits")))
	}
	day.Summary = strings.Join(parts, ", ")
	return day, nil
}

// pocketWidget is the wardrobe at a glance: today's outfit first, then
// tomorrow's plan, the next trip to pack for and a piece that waits to be
// worn; the laundry, the tailor and what is lent out as numbers. Only what
// is kept here: ideas for today need the weather, so they are left to the app.
func (s *Server) pocketWidget(r *http.Request) (*core.WidgetData, error) {
	ctx := r.Context()
	uid := actorID(ctx)
	all, err := s.store.ListItems(ctx, uid, store.ItemFilter{})
	if err != nil {
		return nil, err
	}
	states := map[string]int{}
	for _, it := range all {
		states[it.Status]++
	}
	out := &core.WidgetData{URL: "/"}
	now, tomorrow := today(), store.AddDays(today(), 1)

	// today: what was worn, else the plan
	wears, err := s.store.ListWears(ctx, uid, now, now)
	if err != nil {
		return nil, err
	}
	var on *store.Wear
	for _, x := range wears {
		if on == nil || !x.Planned {
			on = x
		}
	}
	if on != nil {
		title, pieces := s.wearTitle(ctx, uid, on)
		detail := "Worn today"
		if on.Planned {
			detail = "Planned for today"
		}
		out.Line = "Today: " + firstNonEmpty(pieces, title)
		out.Items = append(out.Items, core.WidgetItem{ID: itoa(on.ID), Title: title, Detail: joinNonEmpty(" · ", detail, occasionLabel(on.Occasion)), URL: "/"})
	}
	if p, err := s.store.PlanFor(ctx, uid, tomorrow); err == nil {
		title, _ := s.wearTitle(ctx, uid, p)
		out.Items = append(out.Items, core.WidgetItem{ID: itoa(p.ID), Title: title, Detail: joinNonEmpty(" · ", "Planned for tomorrow", occasionLabel(p.Occasion)), URL: "/plan?date=" + tomorrow})
	}

	// the next trip to pack for
	packs, err := s.store.ListPacks(ctx, uid)
	if err != nil {
		return nil, err
	}
	for _, p := range packs {
		if p.StartDate >= now || (p.EndDate >= now && p.StartDate != "") {
			when := "Under way"
			if d := daysBetween(now, p.StartDate); d >= 0 {
				when = "Leaving " + whenWords(d)
			}
			out.Items = append(out.Items, core.WidgetItem{ID: itoa(p.ID), Title: firstNonEmpty(p.Name, p.Destination, "Packing list"), Detail: joinNonEmpty(" · ", when, packedWords(p)), URL: "/packing/" + itoa(p.ID)})
			break
		}
	}

	// a piece that has waited a long time
	if it := s.nudge(ctx, uid); it != nil {
		detail := "not worn yet"
		if it.LastWorn != "" {
			detail = "last worn " + niceDate(it.LastWorn)
		}
		out.Items = append(out.Items, core.WidgetItem{ID: itoa(it.ID), Title: it.Name, Detail: "Waiting to be worn · " + detail, URL: "/closet/" + itoa(it.ID)})
	}

	if len(all) == 0 && len(out.Items) == 0 {
		return nil, nil
	}
	var waiting []string
	if n := states["laundry"]; n > 0 {
		waiting = append(waiting, fmt.Sprintf("%d %s in the laundry", n, plural(n, "piece", "pieces")))
	}
	if n := states["repair"]; n > 0 {
		waiting = append(waiting, fmt.Sprintf("%d at the tailor", n))
	}
	if n := states["lent"]; n > 0 {
		waiting = append(waiting, fmt.Sprintf("%d lent out", n))
	}
	if out.Line == "" {
		out.Line = strings.Join(waiting, ", ")
	}
	out.Metrics = []core.Metric{{Label: "In the laundry", Value: itoa(int64(states["laundry"]))}}
	if n := states["repair"]; n > 0 {
		out.Metrics = append(out.Metrics, core.Metric{Label: "At the tailor", Value: itoa(int64(n))})
	}
	if n := states["lent"]; n > 0 {
		out.Metrics = append(out.Metrics, core.Metric{Label: "Lent out", Value: itoa(int64(n))})
	}
	out.Metrics = append(out.Metrics, core.Metric{Label: "Pieces", Value: itoa(int64(len(all)))})
	return out, nil
}
