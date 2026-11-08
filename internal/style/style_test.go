package style

import "testing"

func TestFamilies(t *testing.T) {
	cases := map[string]string{"#1f2b48": "navy", "#f6f5f1": "white", "#c0935f": "camel", "#c43a31": "red", "#8b8e94": "grey", "#1b1b1d": "black", "#5f6236": "olive"}
	for hex, want := range cases {
		if got := FamilyOf(hex); got != want {
			t.Errorf("%s = %s, want %s", hex, got, want)
		}
	}
	if (Piece{Kind: "jeans", Colors: []string{"#7d98b8"}}).Family() != "denim" {
		t.Error("light blue jeans are denim")
	}
}

func p(id int64, kind, hex string) Piece {
	k, _ := KindByID(kind)
	return Piece{ID: id, Name: k.Label, Category: k.Category, Kind: kind, Colors: []string{hex}, Pattern: "plain", Warmth: k.Warmth, Formality: k.Formality, Rain: k.Rain, DaysSince: -1, Ready: true}
}

func TestRateWeather(t *testing.T) {
	hot, cold := 30.0, 2.0
	shorts := []Piece{p(1, "tshirt", "#ffffff"), p(2, "shorts", "#1f2b48"), p(3, "sneakers", "#ffffff")}
	summer := Rate(shorts, Context{TMax: &hot, TMin: &hot, Occasion: "everyday"})
	winter := Rate(shorts, Context{TMax: &cold, TMin: &cold, Occasion: "everyday"})
	if summer.Score <= winter.Score+20 {
		t.Fatalf("shorts: summer %v winter %v", summer.Score, winter.Score)
	}
	if len(winter.Warnings) == 0 {
		t.Fatal("no warning for shorts at 2°C")
	}
}

func TestGenerateSkipsLaundryAndRepeats(t *testing.T) {
	all := []Piece{p(1, "shirt", "#ffffff"), p(2, "tshirt", "#1b1b1d"), p(3, "chinos", "#1f2b48"), p(4, "jeans", "#2c3d58"), p(5, "sneakers", "#ffffff"), p(6, "loafers", "#6a4630")}
	all[0].Ready = false
	out := Generate(all, Context{Occasion: "work"}, 4)
	if len(out) == 0 {
		t.Fatal("nothing generated")
	}
	for _, s := range out {
		for _, id := range s.Items {
			if id == 1 {
				t.Fatal("suggested a piece in the laundry")
			}
		}
	}
	if n, _ := Count(all, Context{Occasion: "work"}, 0, 3, 0); n == 0 {
		t.Fatal("no outfits with the chinos")
	}
}

func TestOccasionFor(t *testing.T) {
	cases := map[string]string{"Client presentation": "formal_work", "Vjenčanje Amre i Tarika": "wedding", "Trening": "sport", "Dinner with Lejla": "dinner", "Dentist": ""}
	for title, want := range cases {
		if got := OccasionFor(title); got != want {
			t.Errorf("%q = %q, want %q", title, got, want)
		}
	}
}
