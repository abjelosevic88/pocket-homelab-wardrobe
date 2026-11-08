package server

import (
	"context"
	"math/rand"
	"time"

	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/style"
)

// ---- DEMO_DATA: a believable wardrobe with four months of wearing ----

type demoPiece struct {
	name, kind, hex, pattern, material, brand, size, shop string
	price                                                 int64
	months                                                int // bought this many months ago
	styles                                                []string
	seasons                                               []string
	fav                                                   bool
}

var demoPieces = []demoPiece{
	{"White oxford shirt", "shirt", "#f4f3ee", "plain", "cotton", "Uniqlo", "M", "Uniqlo", 3990, 20, []string{"smart_casual", "classic"}, nil, true},
	{"Light blue shirt", "shirt", "#a9c6e3", "plain", "cotton", "COS", "M", "COS", 5900, 14, []string{"smart_casual", "business"}, nil, false},
	{"Blue striped shirt", "shirt", "#6f8fb8", "stripes", "cotton", "Massimo Dutti", "M", "Massimo Dutti", 4995, 9, []string{"smart_casual", "preppy"}, []string{"spring", "summer", "autumn"}, false},
	{"Denim shirt", "shirt", "#4f6f93", "plain", "denim", "Levi's", "M", "Levi's", 6500, 30, []string{"casual"}, nil, false},
	{"White T-shirt", "tshirt", "#f7f7f5", "plain", "cotton", "Uniqlo", "M", "Uniqlo", 1490, 6, []string{"minimal", "casual"}, nil, true},
	{"Black T-shirt", "tshirt", "#1c1c1e", "plain", "cotton", "Uniqlo", "M", "Uniqlo", 1490, 6, []string{"minimal", "edgy"}, nil, false},
	{"Grey marl T-shirt", "tshirt", "#9a9ca0", "plain", "cotton", "Arket", "M", "Arket", 1900, 11, []string{"casual"}, nil, false},
	{"Navy polo shirt", "polo", "#22304f", "plain", "cotton piqué", "Lacoste", "M", "Lacoste", 8900, 22, []string{"preppy", "smart_casual"}, []string{"spring", "summer"}, false},
	{"Cream long-sleeve top", "longsleeve", "#ece2cb", "plain", "cotton", "COS", "M", "COS", 3500, 8, []string{"minimal"}, nil, false},
	{"Grey crew-neck jumper", "sweater", "#8c8f95", "plain", "merino", "Uniqlo", "M", "Uniqlo", 3990, 13, []string{"smart_casual", "minimal"}, []string{"autumn", "winter", "spring"}, true},
	{"Navy cable knit", "sweater", "#223050", "plain", "wool", "Massimo Dutti", "M", "Massimo Dutti", 7995, 25, []string{"smart_casual", "classic"}, []string{"autumn", "winter"}, false},
	{"Camel turtleneck", "turtleneck", "#b98d5a", "plain", "merino", "COS", "M", "COS", 6900, 4, []string{"classic", "minimal"}, []string{"autumn", "winter"}, true},
	{"Burgundy cardigan", "cardigan", "#6d2433", "plain", "wool", "Arket", "M", "Arket", 7900, 16, []string{"preppy"}, []string{"autumn", "winter"}, false},
	{"Grey hoodie", "hoodie", "#7e8186", "plain", "cotton", "Nike", "M", "Sport Vision", 5500, 28, []string{"casual", "sporty"}, nil, false},
	{"Dark straight jeans", "jeans", "#2c3d58", "plain", "denim", "Levi's", "32/32", "Levi's", 9900, 18, []string{"smart_casual", "casual"}, nil, true},
	{"Light blue jeans", "jeans", "#7d98b8", "plain", "denim", "Levi's", "32/32", "Zalando", 8900, 10, []string{"casual"}, []string{"spring", "summer", "autumn"}, false},
	{"Black jeans", "jeans", "#202022", "plain", "denim", "Weekday", "32", "Zalando", 6000, 15, []string{"edgy", "minimal"}, nil, false},
	{"Navy chinos", "chinos", "#25314d", "plain", "cotton", "Massimo Dutti", "32", "Massimo Dutti", 5995, 12, []string{"smart_casual", "preppy"}, nil, true},
	{"Stone chinos", "chinos", "#cdbb98", "plain", "cotton", "Uniqlo", "32", "Uniqlo", 3990, 7, []string{"smart_casual", "preppy"}, []string{"spring", "summer", "autumn"}, false},
	{"Grey wool trousers", "trousers", "#6c6f75", "plain", "wool", "COS", "32", "COS", 8900, 19, []string{"classic", "business"}, []string{"autumn", "winter", "spring"}, false},
	{"Olive cargo trousers", "cargo", "#5c5f37", "plain", "cotton", "Carhartt", "32", "Zalando", 9500, 5, []string{"street", "outdoor"}, nil, false},
	{"Navy shorts", "shorts", "#26324f", "plain", "cotton", "Uniqlo", "M", "Uniqlo", 2490, 15, []string{"casual"}, []string{"summer"}, false},
	{"Navy blazer", "blazer", "#202b46", "plain", "wool", "Massimo Dutti", "50", "Massimo Dutti", 17995, 24, []string{"smart_casual", "classic", "business"}, nil, true},
	{"Charcoal wool coat", "coat", "#3a3d43", "plain", "wool", "COS", "50", "COS", 24900, 26, []string{"classic", "minimal"}, []string{"autumn", "winter"}, false},
	{"Camel coat", "coat", "#c0935f", "plain", "wool", "Massimo Dutti", "50", "Massimo Dutti", 27995, 2, []string{"smart_casual", "classic"}, []string{"autumn", "winter"}, true},
	{"Beige trench coat", "trench", "#d2bd97", "plain", "cotton", "Zara", "M", "Zara", 11995, 17, []string{"classic"}, []string{"spring", "autumn"}, false},
	{"Black puffer", "puffer", "#1d1d20", "plain", "nylon", "Uniqlo", "M", "Uniqlo", 9990, 23, []string{"street", "casual"}, []string{"winter"}, false},
	{"Denim jacket", "denim_jacket", "#4b6b8e", "plain", "denim", "Levi's", "M", "Levi's", 9900, 34, []string{"casual"}, []string{"spring", "autumn"}, false},
	{"Olive rain jacket", "raincoat", "#5e6236", "plain", "nylon", "The North Face", "M", "Intersport", 12900, 21, []string{"outdoor"}, []string{"spring", "autumn"}, false},
	{"White leather trainers", "sneakers", "#f3f2ee", "plain", "leather", "Veja", "43", "Zalando", 13000, 14, []string{"smart_casual", "minimal", "casual"}, nil, true},
	{"Grey running shoes", "running_shoes", "#8e9196", "plain", "mesh", "New Balance", "43", "Sport Vision", 11000, 9, []string{"sporty"}, nil, false},
	{"Brown suede loafers", "loafers", "#6b4630", "plain", "suede", "Geox", "43", "Geox", 12900, 20, []string{"smart_casual", "preppy"}, []string{"spring", "summer", "autumn"}, false},
	{"Black smart shoes", "smart_shoes", "#1b1b1d", "plain", "leather", "Ecco", "43", "Ecco", 16000, 30, []string{"business", "classic"}, nil, false},
	{"Brown Chelsea boots", "boots", "#5e3e2a", "plain", "leather", "Timberland", "43", "Zalando", 18000, 15, []string{"smart_casual", "classic"}, []string{"autumn", "winter", "spring"}, true},
	{"Winter boots", "winter_boots", "#4a3a2c", "plain", "leather", "Timberland", "43", "Humanic", 17000, 25, []string{"outdoor"}, []string{"winter"}, false},
	{"Brown leather belt", "belt", "#6a4630", "plain", "leather", "Massimo Dutti", "90", "Massimo Dutti", 3995, 24, []string{"smart_casual", "classic"}, nil, false},
	{"Grey wool scarf", "scarf", "#7f8288", "plain", "wool", "Arket", "", "Arket", 3500, 13, []string{"classic"}, []string{"autumn", "winter"}, false},
	{"Navy beanie", "beanie", "#22304f", "plain", "wool", "Arket", "", "Arket", 1900, 13, []string{"casual"}, []string{"winter"}, false},
	{"Black backpack", "backpack", "#1d1d1f", "plain", "nylon", "Rains", "", "Zalando", 8900, 20, []string{"minimal"}, nil, false},
	{"Tan leather bag", "bag", "#a8774a", "plain", "leather", "Mango", "", "Mango", 7999, 11, []string{"smart_casual", "classic"}, nil, false},
	{"Steel watch", "watch", "#b9bcc1", "plain", "steel", "Seiko", "", "Seiko", 22000, 40, []string{"classic"}, nil, true},
	{"Checked flannel shirt", "shirt", "#7a2f2f", "checks", "flannel", "Uniqlo", "M", "Uniqlo", 2990, 27, []string{"casual", "outdoor"}, []string{"autumn", "winter"}, false},
	{"Orange knit", "sweater", "#d9742c", "plain", "wool", "Zara", "M", "Zara", 3995, 13, nil, []string{"autumn", "winter"}, false},
}

func (s *Server) seedDemo(ctx context.Context, uid int64) error {
	ctx = context.WithoutCancel(ctx)
	if have, err := s.store.ListItems(ctx, uid, store.ItemFilter{}); err != nil || len(have) > 0 {
		return err
	}
	rnd := rand.New(rand.NewSource(42))
	st := s.settingsOf(ctx, uid)
	st.Styles = []string{"smart_casual", "minimal"}
	st.LoveColors, st.AvoidColors = []string{"navy", "camel", "grey"}, []string{"orange"}
	st.SizeTop, st.SizeBottom, st.SizeShoe = "M", "32/32", "43"
	st.Budget, st.Currency = 15000, "EUR"
	st.FitNotes = "Straight or slim, not skinny. Likes things that look smart without a tie."
	st.HomeName = "Sarajevo"
	_ = s.store.SaveUserSettings(ctx, uid, st)

	now := time.Now()
	var ids []int64
	for _, d := range demoPieces {
		k, _ := style.KindByID(d.kind)
		price := d.price
		bought := now.AddDate(0, -d.months, -rnd.Intn(20)).Format("2006-01-02")
		it := &store.Item{UserID: uid, Name: d.name, Category: k.Category, Kind: d.kind, Colors: []store.Color{{Hex: d.hex, Name: style.FamilyOf(d.hex)}}, Pattern: d.pattern,
			Material: d.material, Brand: d.brand, Size: d.size, Shop: d.shop, Price: &price, Currency: "EUR", BoughtOn: bought, Warmth: k.Warmth, Formality: k.Formality, Rain: k.Rain,
			Styles: d.styles, Seasons: d.seasons, Favourite: d.fav, Status: "ready", Source: "demo", SourceRef: d.name}
		if it.Styles == nil {
			it.Styles = []string{}
		}
		if it.Seasons == nil {
			it.Seasons = []string{}
		}
		if d.kind == "sweater" && d.material == "merino" {
			it.Care = "Wash cold, dry flat."
		}
		if err := s.store.CreateItem(ctx, it); err != nil {
			return err
		}
		ids = append(ids, it.ID)
	}
	// four months of wearing, chosen by the engine for each day's weather
	pieces, _ := s.pieces(ctx, uid, today())
	c := style.Context{Styles: st.Styles, Style: "smart_casual", Avoid: st.AvoidColors, Love: st.LoveColors}
	last := map[int64]int{}
	for back := 120; back >= 1; back-- {
		day := now.AddDate(0, 0, -back)
		date := day.Format("2006-01-02")
		if rnd.Intn(10) < 2 {
			continue // not every day is logged
		}
		w := demoWeather(date, 43.86)
		cc := c
		w.applyTo(&cc)
		cc.Season = style.SeasonOf(day)
		cc.Occasion = "work"
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			cc.Occasion = "everyday"
			cc.Style = "minimal"
		}
		cc.Shuffle = int64(back)
		for i := range pieces {
			pieces[i].DaysSince = -1
			if d, ok := last[pieces[i].ID]; ok {
				pieces[i].DaysSince = d - back
			}
			pieces[i].Ready = true
		}
		sugg := style.Generate(pieces, cc, 4)
		if len(sugg) == 0 {
			continue
		}
		pick := sugg[rnd.Intn(len(sugg))]
		for _, id := range pick.Items {
			last[id] = back
		}
		x := &store.Wear{UserID: uid, Date: date, ItemIDs: pick.Items, Occasion: cc.Occasion, Feel: 3 + rnd.Intn(3), Source: "demo"}
		x.Weather = []byte(toJSONs(map[string]any{"t_max": w.TMax, "t_min": w.TMin, "condition": w.Condition, "code": w.Code, "place": "Sarajevo"}))
		if err := s.store.CreateWear(ctx, x); err != nil {
			return err
		}
	}
	// saved outfits
	byName := map[string]int64{}
	for i, d := range demoPieces {
		byName[d.name] = ids[i]
	}
	outfits := []struct {
		name, occ, style string
		pieces           []string
		rating           int
		fav              bool
	}{
		{"The office classic", "work", "smart_casual", []string{"Navy blazer", "White oxford shirt", "Grey wool trousers", "Brown suede loafers", "Brown leather belt"}, 5, true},
		{"Camel and grey", "work", "classic", []string{"Camel coat", "Grey crew-neck jumper", "Navy chinos", "Brown Chelsea boots"}, 5, true},
		{"Easy Saturday", "everyday", "minimal", []string{"White T-shirt", "Dark straight jeans", "White leather trainers", "Denim jacket"}, 4, false},
		{"Dinner out", "dinner", "smart_casual", []string{"Camel turtleneck", "Black jeans", "Brown Chelsea boots", "Charcoal wool coat"}, 5, true},
		{"Rainy commute", "work", "smart_casual", []string{"Light blue shirt", "Navy cable knit", "Navy chinos", "Olive rain jacket", "Brown Chelsea boots"}, 4, false},
		{"Summer smart", "work", "preppy", []string{"Navy polo shirt", "Stone chinos", "Brown suede loafers"}, 4, false},
		{"Big meeting", "formal_work", "business", []string{"Navy blazer", "Light blue shirt", "Grey wool trousers", "Black smart shoes"}, 5, false},
		{"Gym", "sport", "sporty", []string{"Grey hoodie", "Black T-shirt", "Grey running shoes", "Navy shorts"}, 3, false},
	}
	for _, o := range outfits {
		var list []int64
		for _, n := range o.pieces {
			list = append(list, byName[n])
		}
		ot := &store.Outfit{UserID: uid, Name: o.name, ItemIDs: list, Occasion: o.occ, Style: o.style, Rating: o.rating, Favourite: o.fav, MadeBy: "me", Seasons: []string{}}
		if err := s.store.CreateOutfit(ctx, ot); err != nil {
			return err
		}
	}
	// the laundry basket, the tailor, a lent jacket
	_, _ = s.store.SetItemStatus(ctx, uid, []int64{byName["Grey marl T-shirt"], byName["Black T-shirt"], byName["Blue striped shirt"], byName["Grey hoodie"]}, "laundry", "")
	_, _ = s.store.SetItemStatus(ctx, uid, []int64{byName["Grey wool trousers"]}, "repair", "hem at the tailor")
	_, _ = s.store.SetItemStatus(ctx, uid, []int64{byName["Denim jacket"]}, "lent", "Tarik has it")
	// tomorrow's plan
	tomorrow := store.AddDays(today(), 1)
	_ = s.store.CreateWear(ctx, &store.Wear{UserID: uid, Date: tomorrow, Planned: true, ItemIDs: []int64{byName["Navy blazer"], byName["White oxford shirt"], byName["Navy chinos"], byName["Brown Chelsea boots"]}, Occasion: "formal_work", Event: "Client presentation", Source: "demo"})
	// ideas kept, a trip to pack for
	_ = s.store.CreateIdea(ctx, &store.Idea{UserID: uid, Title: "Grey flannel trousers", Kind: "trousers", Category: "bottom", ColorHex: "#6c6f75", ColorName: "grey", Why: "A second smart trouser while the first is at the tailor.", Unlocks: 12, Status: "wish", MadeBy: "gaps"})
	lat, lng := 41.3874, 2.1686
	pk := &store.Pack{UserID: uid, Name: "Barcelona", Destination: "Barcelona, Spain", Lat: &lat, Lng: &lng, StartDate: store.AddDays(today(), 12), EndDate: store.AddDays(today(), 16), Purpose: "city", TripRef: "7",
		Items:  []store.PackItem{{ItemID: byName["White T-shirt"], Qty: 1, Packed: true}, {ItemID: byName["Navy chinos"], Qty: 1}, {ItemID: byName["White leather trainers"], Qty: 1, Packed: true}},
		Extras: []store.PackExtra{{Text: "Socks × 6"}, {Text: "Sunglasses", Packed: true}}}
	pk.Weather = []byte(toJSONs(demoWeather(pk.StartDate, lat)))
	_ = s.store.CreatePack(ctx, pk)
	s.log.Info("demo wardrobe seeded", "pieces", len(ids))
	return nil
}

func demoEvents(date string) []dayEvent {
	t, _ := store.ParseDate(date)
	var out []dayEvent
	switch t.Weekday() {
	case time.Monday, time.Wednesday:
		out = append(out, dayEvent{Title: "Team meeting", Time: "10:00", EndTime: "11:00", Ref: "d1"})
	case time.Tuesday:
		out = append(out, dayEvent{Title: "Client presentation", Time: "14:00", EndTime: "15:30", Place: "Office", Ref: "d2"})
	case time.Thursday:
		out = append(out, dayEvent{Title: "1:1 with Emir", Time: "09:30", EndTime: "10:00", Ref: "d3"})
	case time.Friday:
		out = append(out, dayEvent{Title: "Dinner with Lejla", Time: "20:00", EndTime: "22:30", Place: "Restoran Kibe", Ref: "d4"})
	case time.Saturday:
		out = append(out, dayEvent{Title: "Amra's birthday party", Time: "19:00", Ref: "d5"})
	}
	for i := range out {
		out[i].Occasion = style.OccasionFor(out[i].Title + " " + out[i].Place)
	}
	if out == nil {
		out = []dayEvent{}
	}
	return out
}

func demoPurchases() []purchase {
	p := func(v int64) *int64 { return &v }
	return []purchase{
		{App: "parcels", Ref: "p31", Date: store.AddDays(today(), -1), Title: "Zalando order", Shop: "Zalando", Amount: p(8995), Currency: "EUR", Status: "delivered", ReturnBy: store.AddDays(today(), 27)},
		{App: "parcels", Ref: "p32", Date: store.AddDays(today(), 2), Title: "COS — knitwear", Shop: "COS", Amount: p(6900), Currency: "EUR", Status: "in_transit", ExpectedOn: store.AddDays(today(), 2)},
		{App: "money", Ref: "t901", Date: store.AddDays(today(), -9), Title: "Uniqlo", Shop: "Uniqlo", Amount: p(2980), Currency: "EUR"},
		{App: "money", Ref: "t877", Date: store.AddDays(today(), -24), Title: "Deichmann", Shop: "Deichmann", Amount: p(4999), Currency: "EUR"},
	}
}

func demoWishes() []map[string]any {
	return []map[string]any{
		{"ref": "w1", "title": "Merino quarter-zip, navy", "store": "Arket", "price": 7900, "currency": "EUR"},
		{"ref": "w2", "title": "Suede bomber jacket", "store": "Zara", "price": 12995, "currency": "EUR"},
	}
}
