package server

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
)

// closet adds a small wardrobe for tok and returns the ids by name.
func closet(a *app, tok string) map[string]int64 {
	a.t.Helper()
	out := map[string]int64{}
	for _, p := range []map[string]any{
		{"name": "White shirt", "kind": "shirt", "colors": []map[string]any{{"hex": "#f5f5f2"}}},
		{"name": "Grey jumper", "kind": "sweater", "colors": []map[string]any{{"name": "grey"}}},
		{"name": "Navy chinos", "kind": "chinos", "colors": []map[string]any{{"hex": "#22304f"}}, "price": 6000},
		{"name": "Dark jeans", "kind": "jeans", "colors": []map[string]any{{"hex": "#2c3d58"}}},
		{"name": "White trainers", "kind": "sneakers", "colors": []map[string]any{{"name": "white"}}},
		{"name": "Brown boots", "kind": "boots", "colors": []map[string]any{{"name": "brown"}}},
		{"name": "Camel coat", "kind": "coat", "colors": []map[string]any{{"name": "camel"}}},
	} {
		m := a.expect(201, tok, "POST", "/api/v1/items", p)
		out[p["name"].(string)] = int64(num(m, "id"))
	}
	return out
}

func TestPiecesArePerPerson(t *testing.T) {
	a := newApp(t, nil)
	ids := closet(a, a.alice)
	shirt := itoa(ids["White shirt"])
	it := a.ok(a.alice, "GET", "/api/v1/items/"+shirt, nil)["item"].(map[string]any)
	if it["category"] != "top" || num(it, "formality") != 3 {
		t.Fatalf("kind defaults: %v", it)
	}
	if c := it["colors"].([]any)[0].(map[string]any); c["name"] != "white" {
		t.Fatalf("colour family: %v", c)
	}
	// Bob sees none of it, and cannot touch it
	if l := a.ok(a.bob, "GET", "/api/v1/items", nil)["items"].([]any); len(l) != 0 {
		t.Fatalf("bob sees %d pieces", len(l))
	}
	a.expect(404, a.bob, "GET", "/api/v1/items/"+shirt, nil)
	a.expect(404, a.bob, "PUT", "/api/v1/items/"+shirt, map[string]any{"name": "Mine"})
	a.expect(404, a.bob, "DELETE", "/api/v1/items/"+shirt, nil)
	a.expect(400, a.bob, "POST", "/api/v1/outfits", map[string]any{"item_ids": []int64{ids["White shirt"], ids["Navy chinos"]}})
	if m := a.ok(a.bob, "POST", "/api/v1/items/status", map[string]any{"ids": []int64{ids["White shirt"]}, "status": "laundry"}); num(m, "changed") != 0 {
		t.Fatalf("bob moved alice's shirt")
	}
	// an outfit and a wear for Alice; Bob gets 404s
	o := a.expect(201, a.alice, "POST", "/api/v1/outfits", map[string]any{"item_ids": []int64{ids["White shirt"], ids["Navy chinos"], ids["White trainers"]}})
	a.expect(404, a.bob, "GET", "/api/v1/outfits/"+id(o), nil)
	w := a.expect(201, a.alice, "POST", "/api/v1/wears", map[string]any{"item_ids": []int64{ids["White shirt"], ids["Navy chinos"]}})
	a.expect(404, a.bob, "PUT", "/api/v1/wears/"+id(w), map[string]any{"notes": "x"})
	// logging a wear puts the shirt in the laundry, and counts it
	it = a.ok(a.alice, "GET", "/api/v1/items/"+shirt, nil)["item"].(map[string]any)
	if it["status"] != "laundry" || num(it, "wears") != 1 {
		t.Fatalf("after wearing: %v %v", it["status"], it["wears"])
	}
	chinos := a.ok(a.alice, "GET", "/api/v1/items/"+itoa(ids["Navy chinos"]), nil)["item"].(map[string]any)
	if num(chinos, "per_wear") != 6000 {
		t.Fatalf("cost per wear %v", chinos["per_wear"])
	}
	// a plan for the past is refused, for tomorrow it is fine
	a.expect(400, a.alice, "POST", "/api/v1/wears", map[string]any{"item_ids": []int64{ids["Dark jeans"]}, "planned": true, "date": "2020-01-01"})
	a.expect(201, a.alice, "POST", "/api/v1/wears", map[string]any{"item_ids": []int64{ids["Dark jeans"]}, "planned": true, "date": dayAfter(1)})
	if s := a.ok(a.alice, "GET", "/api/v1/summary", nil); s["planned_tomorrow"] != true || num(s, "laundry") != 1 {
		t.Fatalf("summary %v", s)
	}
}

func dayAfter(n int) string { return store.AddDays(today(), n) }

func TestSuggestionsAndShopping(t *testing.T) {
	a := newApp(t, nil)
	ids := closet(a, a.alice)
	tmax, tmin := 8.0, 1.0
	res := a.ok(a.alice, "POST", "/api/v1/suggest", map[string]any{"occasion": "work", "t_max": tmax, "t_min": tmin, "n": 4})
	sugg := res["suggestions"].([]any)
	if len(sugg) == 0 {
		t.Fatalf("no suggestions: %v", res)
	}
	first := sugg[0].(map[string]any)
	hasCoat := false
	for _, x := range first["items"].([]any) {
		if int64(x.(float64)) == ids["Camel coat"] {
			hasCoat = true
		}
	}
	if !hasCoat {
		t.Fatalf("at 1–8°C the first outfit should have the coat: %v", first)
	}
	// "should I buy this?": another white shirt is a near twin
	v := a.ok(a.alice, "POST", "/api/v1/check", map[string]any{"kind": "shirt", "color_name": "white"})["verdict"].(map[string]any)
	if len(v["similar"].([]any)) != 1 {
		t.Fatalf("check similar: %v", v)
	}
	// the gaps for minimal: a black T-shirt is missing, and makes outfits
	g := a.ok(a.alice, "GET", "/api/v1/gaps?style=minimal", nil)["ideas"].([]any)
	if len(g) == 0 {
		t.Fatalf("no gaps")
	}
	// Bob's empty closet gives nothing to suggest, without failing
	if r := a.ok(a.bob, "POST", "/api/v1/suggest", map[string]any{}); r["empty"] == "" {
		t.Fatalf("bob: %v", r)
	}
}

func TestPhotosArePerPerson(t *testing.T) {
	a := newApp(t, nil)
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for x := 0; x < 40; x++ {
		for y := 0; y < 40; y++ {
			img.Set(x, y, color.RGBA{30, 40, 70, 255})
		}
	}
	var pic bytes.Buffer
	_ = jpeg.Encode(&pic, img, nil)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "p.jpg")
	_, _ = fw.Write(pic.Bytes())
	_ = mw.Close()
	req, _ := http.NewRequest("POST", a.url+"/api/v1/photos", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+a.alice)
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != 201 {
		t.Fatalf("upload: %v %v", err, res.StatusCode)
	}
	var p map[string]any
	_ = json.NewDecoder(res.Body).Decode(&p)
	res.Body.Close()
	pid := int64(num(p, "id"))
	a.expect(200, a.alice, "GET", "/api/v1/photos/"+itoa(pid), nil)
	a.expect(404, a.bob, "GET", "/api/v1/photos/"+itoa(pid), nil)
	// Bob cannot put Alice's photo on his own piece
	a.expect(400, a.bob, "POST", "/api/v1/items", map[string]any{"kind": "tshirt", "photo_id": pid})
	m := a.expect(201, a.alice, "POST", "/api/v1/items", map[string]any{"kind": "tshirt", "photo_id": pid})
	if m["thumb_url"] == "" {
		t.Fatalf("no thumb url: %v", m)
	}
	// a collage link opens without signing in, and only with its signature
	o := a.expect(201, a.alice, "POST", "/api/v1/outfits", map[string]any{"item_ids": []int64{int64(num(m, "id"))}})
	share := a.ok(a.alice, "GET", "/api/v1/outfits/"+id(o), nil)["share"].(string)
	path := share[len("http://wardrobe.test"):]
	r2, _ := http.Get(a.url + path)
	if r2.StatusCode != 200 || r2.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("shared collage: %d %s", r2.StatusCode, r2.Header.Get("Content-Type"))
	}
	r2.Body.Close()
	r3, _ := http.Get(a.url + path[:len(path)-8] + "00000000.jpg")
	if r3.StatusCode != 404 {
		t.Fatalf("forged signature answered %d", r3.StatusCode)
	}
	r3.Body.Close()
}

func TestLinkScopes(t *testing.T) {
	r := func(method, path string) *http.Request { return httptest.NewRequest(method, path, nil) }
	read, write := []string{"wardrobe:read"}, []string{"wardrobe:write"}
	cases := []struct {
		scopes []string
		req    *http.Request
		want   bool
	}{
		{read, r("GET", "/api/v1/items"), true},
		{read, r("GET", "/api/v1/items/3"), true},
		{read, r("GET", "/api/v1/today"), true},
		{read, r("GET", "/api/v1/day/2026-11-08"), true},
		{read, r("POST", "/api/v1/suggest"), true},
		{read, r("GET", "/api/v1/photos/2"), true},
		{read, r("POST", "/api/v1/wears"), false},
		{write, r("POST", "/api/v1/wears"), true},
		{write, r("POST", "/api/v1/items"), true},
		{write, r("PUT", "/api/v1/outfits/4"), true},
		{write, r("GET", "/api/v1/items"), false},
		{append(read, write...), r("POST", "/api/v1/ai/tag"), false},
		{append(read, write...), r("POST", "/api/v1/items/status"), false},
		{append(read, write...), r("POST", "/api/v1/apps/chat"), false},
		{nil, r("GET", "/api/v1/auth/me"), true},
		{read, r("GET", "/api/v1/settings"), false},
		{read, r("GET", "/api/v1/export.json"), false},
	}
	for _, c := range cases {
		if got := linkAllows(c.scopes, c.req); got != c.want {
			t.Errorf("%v %s %s = %v, want %v", c.scopes, c.req.Method, c.req.URL.Path, got, c.want)
		}
	}
}

func TestExportImportTwice(t *testing.T) {
	a := newApp(t, nil)
	ids := closet(a, a.alice)
	a.expect(201, a.alice, "POST", "/api/v1/outfits", map[string]any{"item_ids": []int64{ids["White shirt"], ids["Dark jeans"]}})
	a.expect(201, a.alice, "POST", "/api/v1/wears", map[string]any{"item_ids": []int64{ids["Grey jumper"], ids["Dark jeans"]}})
	a.expect(201, a.alice, "POST", "/api/v1/packs", map[string]any{"name": "Rome", "items": []map[string]any{{"item_id": ids["Dark jeans"], "qty": 1}}})
	req, _ := http.NewRequest("GET", a.url+"/api/v1/export.json", nil)
	req.Header.Set("Authorization", "Bearer "+a.alice)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	_ = json.NewDecoder(res.Body).Decode(&doc)
	res.Body.Close()
	if len(doc["items"].([]any)) != 7 {
		t.Fatalf("export items %d", len(doc["items"].([]any)))
	}
	// into Bob's account twice: the second adds nothing
	first := a.ok(a.bob, "POST", "/api/v1/import", doc)["added"].(map[string]any)
	if num(first, "items") != 7 || num(first, "outfits") != 1 || num(first, "wears") != 1 || num(first, "packs") != 1 {
		t.Fatalf("first import %v", first)
	}
	second := a.ok(a.bob, "POST", "/api/v1/import", doc)["added"].(map[string]any)
	if len(second) != 0 {
		t.Fatalf("second import added %v", second)
	}
	if l := a.ok(a.alice, "GET", "/api/v1/items", nil)["items"].([]any); len(l) != 7 {
		t.Fatalf("alice now has %d", len(l))
	}
}

func TestSettingsArePersonal(t *testing.T) {
	a := newApp(t, nil)
	a.ok(a.alice, "PUT", "/api/v1/settings", map[string]any{"styles": []string{"classic", "nope"}, "avoid_colors": []string{"orange", "plaid"}})
	s := a.ok(a.alice, "GET", "/api/v1/settings", nil)
	if st := s["styles"].([]any); len(st) != 1 || st[0] != "classic" {
		t.Fatalf("styles %v", st)
	}
	if b := a.ok(a.bob, "GET", "/api/v1/settings", nil)["styles"].([]any); len(b) != 0 {
		t.Fatalf("bob got alice's styles: %v", b)
	}
	a.expect(400, a.alice, "PUT", "/api/v1/settings", map[string]any{"shops": []map[string]any{{"name": "X", "url": "https://x.example/search"}}})
}
