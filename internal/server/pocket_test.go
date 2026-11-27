package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/config"
)

// pocketLinks starts Wardrobe behind a fake Core whose pcx_<who>_<search|day|
// widget|capture|read> tokens are links acting for alice@ or bob@ with that
// one scope. The second value is the last manifest Wardrobe registered with.
func pocketLinks(t *testing.T) (*app, func() map[string]any) {
	t.Helper()
	var mu sync.Mutex
	var manifest map[string]any
	coreSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/apps/register":
			var m map[string]any
			_ = json.NewDecoder(r.Body).Decode(&m)
			mu.Lock()
			manifest = m
			mu.Unlock()
			io.WriteString(w, `{}`)
		case "/api/v1/auth/introspect":
			var in struct {
				Token string `json:"token"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			parts := strings.SplitN(strings.TrimPrefix(in.Token, "pcx_"), "_", 2)
			scope := ""
			if len(parts) == 2 {
				scope = map[string]string{"search": core.ScopeSearch, "day": core.ScopeDay, "widget": core.ScopeWidget, "capture": core.ScopeCapture, "read": "wardrobe:read"}[parts[1]]
			}
			if scope == "" {
				io.WriteString(w, `{"active":false}`)
				return
			}
			email := parts[0] + "@example.com"
			fmt.Fprintf(w, `{"active":true,"kind":"delegated","scope":%q,"audience":"wardrobe","user":{"id":1,"sub":%q,"email":%q,"name":%q,"role":"member"},"app":{"id":7,"slug":"search","name":"Pocket Search"}}`, scope, email, email, parts[0])
		default:
			io.WriteString(w, `{}`)
		}
	}))
	t.Cleanup(coreSrv.Close)
	a := newApp(t, func(c *config.Config) { c.CoreURL, c.CoreRegistrationKey = coreSrv.URL, "key" })
	return a, func() map[string]any {
		mu.Lock()
		defer mu.Unlock()
		return manifest
	}
}

// wardrobe gives Alice a closet, a named outfit she wore today, a plan for
// tomorrow, a trip to pack for and an idea she kept.
func wardrobe(a *app) map[string]int64 {
	a.t.Helper()
	ids := closet(a, a.alice)
	look := []int64{ids["White shirt"], ids["Navy chinos"], ids["White trainers"]}
	o := a.expect(201, a.alice, "POST", "/api/v1/outfits", map[string]any{"name": "Friday navy", "occasion": "work", "item_ids": look})
	ids["outfit"] = int64(num(o, "id"))
	w := a.expect(201, a.alice, "POST", "/api/v1/wears", map[string]any{"item_ids": look, "occasion": "work", "notes": "Felt good"})
	ids["worn"] = int64(num(w, "id"))
	p := a.expect(201, a.alice, "POST", "/api/v1/wears", map[string]any{"item_ids": []int64{ids["Grey jumper"], ids["Dark jeans"]}, "planned": true, "date": dayAfter(1), "occasion": "dinner"})
	ids["plan"] = int64(num(p, "id"))
	pack := a.expect(201, a.alice, "POST", "/api/v1/packs", map[string]any{"name": "Lisbon in spring", "destination": "Lisbon", "start_date": dayAfter(3), "end_date": dayAfter(8),
		"items": []map[string]any{{"item_id": ids["Camel coat"], "qty": 1, "packed": true}, {"item_id": ids["Brown boots"], "qty": 1}}})
	ids["pack"] = int64(num(pack, "id"))
	idea := a.expect(201, a.alice, "POST", "/api/v1/ideas", map[string]any{"title": "Navy blazer", "kind": "blazer", "color_name": "navy", "unlocks": 9, "url": "https://shop.example/blazer"})
	ids["idea"] = int64(num(idea, "id"))
	return ids
}

func rowOf(list []map[string]any, key, val string) map[string]any {
	for _, r := range list {
		if r[key] == val {
			return r
		}
	}
	return nil
}

func TestPocketSearch(t *testing.T) {
	a := newApp(t, nil)
	ids := wardrobe(a)
	found := rows(a.ok(a.alice, "GET", "/api/v1/pocket/search?q=navy&limit=20", nil), "items")
	// the three with navy in the name first, then the jeans that are navy by colour
	if len(found) != 4 || found[3]["title"] != "Dark jeans" || found[3]["snippet"] != "Jeans · navy" {
		t.Fatalf("navy: %v", found)
	}
	item := rowOf(found, "kind", "item")
	if item == nil || item["title"] != "Navy chinos" || item["id"] != itoa(ids["Navy chinos"]) || item["url"] != "/closet/"+itoa(ids["Navy chinos"]) || item["snippet"] != "Chinos · navy" || item["date"] != today() {
		t.Fatalf("the piece: %v", item)
	}
	outfit := rowOf(found, "kind", "outfit")
	if outfit == nil || outfit["title"] != "Friday navy" || outfit["url"] != "/outfits/"+itoa(ids["outfit"]) || outfit["snippet"] != "Work · White shirt, Navy chinos, White trainers" {
		t.Fatalf("the outfit: %v", outfit)
	}
	idea := rowOf(found, "kind", "idea")
	if idea == nil || idea["title"] != "Navy blazer" || idea["url"] != "/shop" || idea["link"] != "https://shop.example/blazer" || idea["snippet"] != "Blazer · navy · makes 9 new outfits" {
		t.Fatalf("the idea: %v", idea)
	}
	// a piece that waits in the laundry says so; a packing list is found by where it goes
	if shirt := rows(a.ok(a.alice, "GET", "/api/v1/pocket/search?q=white+shirt", nil), "items"); len(shirt) != 1 || shirt[0]["snippet"] != "Shirt · white · in the laundry" {
		t.Fatalf("the shirt: %v", shirt)
	}
	pack := rows(a.ok(a.alice, "GET", "/api/v1/pocket/search?q=lisbon", nil), "items")
	if len(pack) != 1 || pack[0]["kind"] != "packing list" || pack[0]["title"] != "Lisbon in spring" || pack[0]["url"] != "/packing/"+itoa(ids["pack"]) || pack[0]["date"] != dayAfter(3) || !strings.HasSuffix(pack[0]["snippet"].(string), "1 of 2 packed") {
		t.Fatalf("the packing list: %v", pack)
	}
	// an idea that was hidden is not found any more
	a.ok(a.alice, "PUT", "/api/v1/ideas/"+itoa(ids["idea"]), map[string]any{"status": "dismissed"})
	if l := rows(a.ok(a.alice, "GET", "/api/v1/pocket/search?q=blazer", nil), "items"); len(l) != 0 {
		t.Fatalf("a hidden idea: %v", l)
	}
	// only as many as asked for, nothing for no words, and nothing of Alice's for Bob
	if l := rows(a.ok(a.alice, "GET", "/api/v1/pocket/search?q=white&limit=1", nil), "items"); len(l) != 1 {
		t.Fatalf("limit=1 gave %d", len(l))
	}
	for _, c := range []struct{ tok, q string }{{a.alice, ""}, {a.alice, "   "}, {a.bob, "navy"}, {a.bob, "lisbon"}, {a.bob, "white"}} {
		m := a.ok(c.tok, "GET", "/api/v1/pocket/search?q="+url.QueryEscape(c.q), nil)
		if l, ok := m["items"].([]any); !ok || len(l) != 0 {
			t.Fatalf("q=%q: %v", c.q, m)
		}
	}
	a.expect(401, "", "GET", "/api/v1/pocket/search?q=navy", nil)
}

func TestPocketDay(t *testing.T) {
	a := newApp(t, nil)
	ids := wardrobe(a)
	day := a.ok(a.alice, "GET", "/api/v1/pocket/day/"+today(), nil)
	items := rows(day, "items")
	if day["date"] != today() || day["summary"] != "Wore 1 outfit" || len(items) != 1 {
		t.Fatalf("today: %v", day)
	}
	worn := items[0]
	if worn["id"] != itoa(ids["worn"]) || worn["title"] != "Friday navy" || worn["sub"] != "Work · White shirt, Navy chinos, White trainers" || worn["kind"] != "worn" || worn["note"] != "Felt good" || worn["url"] != "/plan?date="+today() {
		t.Fatalf("worn: %v", worn)
	}
	// the picture is the pieces together, and opens without signing in
	img, _ := worn["image"].(string)
	if !strings.HasPrefix(img, "/s/"+itoa(a.ids["Alice"])+"/") {
		t.Fatalf("image %q", img)
	}
	res, err := http.Get(a.url + img)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("the picture: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	// tomorrow there is a plan, without a saved outfit: its pieces are the line
	day = a.ok(a.alice, "GET", "/api/v1/pocket/day/"+dayAfter(1)+"?tz=Europe/Sarajevo", nil)
	items = rows(day, "items")
	if day["summary"] != "1 outfit planned" || len(items) != 1 || items[0]["title"] != "Grey jumper, Dark jeans" || items[0]["sub"] != "Dinner out" || items[0]["kind"] != "planned" || items[0]["id"] != itoa(ids["plan"]) {
		t.Fatalf("tomorrow: %v", day)
	}
	// a day with nothing, and Alice's day for Bob: only the date
	for _, c := range []struct{ tok, date string }{{a.alice, "2020-01-01"}, {a.bob, today()}, {a.bob, dayAfter(1)}} {
		code, raw := a.raw(c.tok, "GET", "/api/v1/pocket/day/"+c.date, "", nil)
		if want := `{"date":"` + c.date + `","items":[]}`; code != 200 || strings.TrimSpace(string(raw)) != want {
			t.Fatalf("%s: %d %s, want %s", c.date, code, raw, want)
		}
	}
	a.expect(400, a.alice, "GET", "/api/v1/pocket/day/2026-13-40", nil)
	a.expect(401, "", "GET", "/api/v1/pocket/day/"+today(), nil)
}

func TestPocketWidget(t *testing.T) {
	a := newApp(t, nil)
	// nothing yet: nothing to show
	if code, raw := a.raw(a.alice, "GET", "/api/v1/pocket/widget", "", nil); code != 200 || strings.TrimSpace(string(raw)) != `{"items":[]}` {
		t.Fatalf("an empty wardrobe: %d %s", code, raw)
	}
	ids := wardrobe(a)
	a.ok(a.alice, "POST", "/api/v1/items/status", map[string]any{"ids": []int64{ids["Brown boots"]}, "status": "repair"})
	w := a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil)
	items := rows(w, "items")
	if w["line"] != "Today: White shirt, Navy chinos, White trainers" || w["url"] != "/" || len(items) != 3 {
		t.Fatalf("widget: %v", w)
	}
	if r := items[0]; r["id"] != itoa(ids["worn"]) || r["title"] != "Friday navy" || r["detail"] != "Worn today · Work" || r["url"] != "/" {
		t.Fatalf("today's row: %v", r)
	}
	if r := items[1]; r["title"] != "Grey jumper, Dark jeans" || r["detail"] != "Planned for tomorrow · Dinner out" || r["url"] != "/plan?date="+dayAfter(1) {
		t.Fatalf("tomorrow's row: %v", r)
	}
	if r := items[2]; r["title"] != "Lisbon in spring" || r["detail"] != "Leaving in 3 days · 1 of 2 packed" || r["url"] != "/packing/"+itoa(ids["pack"]) {
		t.Fatalf("the trip's row: %v", r)
	}
	metrics := rows(w, "metrics")
	if len(metrics) != 3 || metrics[0]["label"] != "In the laundry" || metrics[0]["value"] != "1" || metrics[1]["label"] != "At the tailor" || metrics[1]["value"] != "1" || metrics[2]["label"] != "Pieces" || metrics[2]["value"] != "7" {
		t.Fatalf("metrics: %v", metrics)
	}
	// it agrees with the sidebar's numbers
	if s := a.ok(a.alice, "GET", "/api/v1/summary", nil); num(s, "laundry") != 1 || num(s, "repair") != 1 || num(s, "items") != 7 || s["worn_today"] != true || s["planned_tomorrow"] != true {
		t.Fatalf("summary: %v", s)
	}
	// Bob has clothes of his own and nothing on today: the line is what waits
	bob := closet(a, a.bob)
	a.ok(a.bob, "POST", "/api/v1/items/status", map[string]any{"ids": []int64{bob["White shirt"], bob["Dark jeans"]}, "status": "laundry"})
	a.ok(a.bob, "POST", "/api/v1/items/status", map[string]any{"ids": []int64{bob["Camel coat"]}, "status": "lent"})
	w = a.ok(a.bob, "GET", "/api/v1/pocket/widget", nil)
	if l, ok := w["items"].([]any); w["line"] != "2 pieces in the laundry, 1 lent out" || !ok || len(l) != 0 {
		t.Fatalf("bob's widget: %v", w)
	}
	if raw, _ := json.Marshal(w); strings.Contains(string(raw), "Friday") || strings.Contains(string(raw), "Lisbon") {
		t.Fatalf("bob sees alice's things: %s", raw)
	}
	// a piece bought long ago and never worn gets a row
	old := a.expect(201, a.bob, "POST", "/api/v1/items", map[string]any{"name": "Linen blazer", "kind": "blazer", "bought_on": "2020-05-01"})
	if l := rows(a.ok(a.bob, "GET", "/api/v1/pocket/widget", nil), "items"); len(l) != 1 || l[0]["title"] != "Linen blazer" || l[0]["detail"] != "Waiting to be worn · not worn yet" || l[0]["url"] != "/closet/"+id(old) {
		t.Fatalf("the piece that waits: %v", l)
	}
	a.expect(401, "", "GET", "/api/v1/pocket/widget", nil)
}

// A link token with one pocket: scope reaches its one route and who it acts
// for, and nothing else in Wardrobe.
func TestPocketLinkScopes(t *testing.T) {
	a, _ := pocketLinks(t)
	wardrobe(a)
	day := "/api/v1/pocket/day/" + today()
	for _, c := range []struct {
		tok, method, path string
		want              int
	}{
		{"pcx_alice_search", "GET", "/api/v1/pocket/search?q=navy", 200},
		{"pcx_alice_search", "GET", "/api/v1/auth/me", 200},
		{"pcx_alice_search", "POST", "/api/v1/pocket/search?q=navy", 405}, // reading only
		{"pcx_alice_search", "GET", day, 403},
		{"pcx_alice_search", "GET", "/api/v1/pocket/widget", 403},
		{"pcx_alice_search", "GET", "/api/v1/search?q=navy", 403},
		{"pcx_alice_search", "GET", "/api/v1/items", 403},
		{"pcx_alice_search", "POST", "/api/v1/items", 403},
		{"pcx_alice_search", "GET", "/api/v1/export.json", 403},
		{"pcx_alice_day", "GET", day, 200},
		{"pcx_alice_day", "GET", day + "?tz=Europe/Sarajevo", 200},
		{"pcx_alice_day", "GET", "/api/v1/day/" + today(), 403},
		{"pcx_alice_day", "GET", "/api/v1/pocket/search?q=navy", 403},
		{"pcx_alice_day", "GET", "/api/v1/pocket/widget", 403},
		{"pcx_alice_day", "GET", "/api/v1/wears", 403},
		{"pcx_alice_widget", "GET", "/api/v1/pocket/widget", 200},
		{"pcx_alice_widget", "GET", "/api/v1/pocket/search?q=navy", 403},
		{"pcx_alice_widget", "GET", day, 403},
		{"pcx_alice_widget", "GET", "/api/v1/summary", 403},
		{"pcx_alice_widget", "GET", "/api/v1/today", 403},
		// nothing can be kept here with a quick line: the route is not there
		{"pcx_alice_capture", "POST", "/api/v1/pocket/capture", 404},
		{"pcx_alice_capture", "GET", "/api/v1/pocket/widget", 403},
		// the app's own scope keeps its own routes, not these
		{"pcx_alice_read", "GET", "/api/v1/search?q=navy", 200},
		{"pcx_alice_read", "GET", "/api/v1/day/" + today(), 200},
		{"pcx_alice_read", "GET", "/api/v1/pocket/search?q=navy", 403},
		{"pcx_alice_read", "GET", day, 403},
		{"pcx_alice_read", "GET", "/api/v1/pocket/widget", 403},
	} {
		if code, body := a.raw(c.tok, c.method, c.path, "application/json", []byte("{}")); code != c.want {
			t.Errorf("%s %s %s → %d, want %d (%s)", c.tok, c.method, c.path, code, c.want, body)
		}
	}
	// the token acts for one person: Alice's things for Alice, none of them for Bob
	if l := rows(a.ok("pcx_alice_search", "GET", "/api/v1/pocket/search?q=navy", nil), "items"); len(l) != 4 {
		t.Fatalf("alice through a link: %v", l)
	}
	if l := rows(a.ok("pcx_bob_search", "GET", "/api/v1/pocket/search?q=navy", nil), "items"); len(l) != 0 {
		t.Fatalf("bob through a link finds alice's: %v", l)
	}
	if d := a.ok("pcx_alice_day", "GET", day, nil); d["summary"] != "Wore 1 outfit" {
		t.Fatalf("alice's day through a link: %v", d)
	}
	if _, raw := a.raw("pcx_bob_day", "GET", day, "", nil); strings.TrimSpace(string(raw)) != `{"date":"`+today()+`","items":[]}` {
		t.Fatalf("bob's day through a link: %s", raw)
	}
	if w := a.ok("pcx_alice_widget", "GET", "/api/v1/pocket/widget", nil); len(rows(w, "items")) != 3 {
		t.Fatalf("alice's widget through a link: %v", w)
	}
	if _, raw := a.raw("pcx_bob_widget", "GET", "/api/v1/pocket/widget", "", nil); strings.TrimSpace(string(raw)) != `{"items":[]}` {
		t.Fatalf("bob's widget through a link: %s", raw)
	}
}

// Core hears what Wardrobe can do, so Search, Day and Hub find it on their own.
func TestManifestCapabilities(t *testing.T) {
	a, manifest := pocketLinks(t)
	if err := a.s.registerWithCore(context.Background()); err != nil {
		t.Fatal(err)
	}
	m := manifest()
	if m["slug"] != "wardrobe" || m["category"] != "Home" {
		t.Fatalf("manifest: %v %v", m["slug"], m["category"])
	}
	search, day, widget := obj(m, "search"), obj(m, "day"), obj(m, "widget")
	if search["what"] == "" || fmt.Sprint(search["kinds"]) != "[item outfit packing list idea]" || fmt.Sprint(search["bangs"]) != "[wardrobe clothes]" {
		t.Fatalf("search: %v", search)
	}
	if day["label"] != "Wardrobe" || day["one"] != "outfit" || day["many"] != "outfits" || day["unit"] != "count" || day["what"] == "" {
		t.Fatalf("day: %v", day)
	}
	if widget["title"] != "What to wear" || widget["what"] == "" {
		t.Fatalf("widget: %v", widget)
	}
	if _, ok := m["capture"]; ok {
		t.Fatalf("capture is not offered: %v", m["capture"])
	}
	offered := map[string]bool{}
	for _, o := range rows(m, "offers") {
		offered[fmt.Sprint(o["scope"])] = true
	}
	for scope, want := range map[string]bool{core.ScopeSearch: true, core.ScopeDay: true, core.ScopeWidget: true, core.ScopeCapture: false, "wardrobe:read": true, "wardrobe:write": true, core.ScopeMoveExport: true} {
		if offered[scope] != want {
			t.Errorf("offer %s = %v, want %v", scope, offered[scope], want)
		}
	}
}
