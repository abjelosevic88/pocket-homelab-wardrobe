package style

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Gap is something worth buying: what it is, why, and how many new
// outfits it would make with what is already there.
type Gap struct {
	Title    string  `json:"title"`
	Category string  `json:"category"`
	Kind     string  `json:"kind"`
	Family   string  `json:"color_name"`
	Hex      string  `json:"color_hex"`
	Why      string  `json:"why"`
	Style    string  `json:"style"`
	Unlocks  int     `json:"unlocks"`
	Examples []Score `json:"examples"` // the idea is item id -1
	Priority float64 `json:"-"`
}

// Virtual makes a piece that is not owned yet (id -1) from a kind and a colour.
func Virtual(kind, family string, styles []string) Piece {
	k, _ := KindByID(kind)
	return Piece{ID: -1, Name: k.Label, Category: k.Category, Kind: kind, Colors: []string{HexOf(family)}, Pattern: "plain",
		Warmth: k.Warmth, Formality: k.Formality, Styles: styles, Rain: k.Rain, DaysSince: -1, Ready: true}
}

// group says which kinds count as "the same thing" when looking for one.
func group(kind string) string {
	switch kind {
	case "tshirt", "tank", "longsleeve", "sports_top":
		return "tee"
	case "sweater", "sweatshirt", "turtleneck":
		return "knit"
	case "hoodie", "fleece":
		return "hoodie"
	case "chinos", "trousers":
		return "trousers"
	case "sneakers", "running_shoes":
		return "sneakers"
	case "smart_shoes", "loafers":
		return "smart"
	case "coat", "trench":
		return "coat"
	case "puffer", "parka":
		return "puffer"
	case "jacket", "denim_jacket", "leather_jacket":
		return "jacket"
	}
	return kind
}

// closeColor says whether two families look alike enough to replace each other.
func closeColor(a, b string) bool {
	if a == b {
		return true
	}
	pairs := [][2]string{{"black", "charcoal"}, {"white", "cream"}, {"beige", "cream"}, {"beige", "khaki"}, {"camel", "brown"}, {"navy", "denim"},
		{"blue", "denim"}, {"light blue", "denim"}, {"olive", "khaki"}, {"pink", "blush"}, {"purple", "lavender"}, {"rust", "orange"}, {"mustard", "yellow"}, {"grey", "silver"}, {"charcoal", "grey"}}
	for _, p := range pairs {
		if (p[0] == a && p[1] == b) || (p[0] == b && p[1] == a) {
			return true
		}
	}
	return false
}

// Similar lists the pieces that are much like p (same kind of thing, a close colour).
func Similar(all []Piece, p Piece) []Piece {
	var out []Piece
	for _, q := range all {
		if q.ID == p.ID || q.Category != p.Category {
			continue
		}
		if group(q.Kind) == group(p.Kind) && closeColor(q.Family(), p.Family()) {
			out = append(out, q)
		}
	}
	return out
}

// Gaps finds what to buy for the person's styles: the staples that are
// missing, ranked by how many new outfits each would make.
func Gaps(all []Piece, c Context, styles []string, limit int) []Gap {
	if len(styles) == 0 {
		styles = []string{"smart_casual"}
	}
	var cands []Gap
	seen := map[string]bool{}
	for si, sid := range styles {
		st, ok := StyleByID(sid)
		if !ok {
			continue
		}
		for i, sp := range st.Staples {
			key := group(sp.Kind) + "|" + sp.Family
			if seen[key] {
				continue
			}
			seen[key] = true
			v := Virtual(sp.Kind, sp.Family, []string{sid})
			if len(Similar(all, v)) > 0 || hasAvoid(c.Avoid, sp.Family) {
				continue
			}
			k, _ := KindByID(sp.Kind)
			cands = append(cands, Gap{Title: sp.Title, Category: k.Category, Kind: sp.Kind, Family: sp.Family, Hex: HexOf(sp.Family), Why: sp.Why, Style: sid,
				Priority: float64(len(st.Staples)-i)*0.3 + float64(len(styles)-si)})
		}
	}
	// what any wardrobe needs, whatever the style
	cnt := map[string]int{}
	rain, warm := false, false
	for _, p := range all {
		cnt[p.Category]++
		if p.Category == "outerwear" && p.Rain {
			rain = true
		}
		if p.Category == "outerwear" && p.Warmth >= 4 {
			warm = true
		}
	}
	basic := func(kind, fam, title, why string, pri float64) {
		key := group(kind) + "|" + fam
		if seen[key] {
			return
		}
		seen[key] = true
		k, _ := KindByID(kind)
		cands = append(cands, Gap{Title: title, Category: k.Category, Kind: kind, Family: fam, Hex: HexOf(fam), Why: why, Style: "", Priority: pri})
	}
	if !rain {
		basic("raincoat", "navy", "A rain jacket", "Nothing you own keeps the rain out.", 2.5)
	}
	if !warm {
		basic("coat", "charcoal", "A warm winter coat", "You have no coat for the cold months.", 2.5)
	}
	if cnt["shoes"] < 2 {
		basic("sneakers", "white", "A second pair of shoes", "With one pair, every outfit ends the same way.", 2)
	}
	if cnt["bottom"] > 0 && cnt["top"] >= cnt["bottom"]*4 {
		basic("chinos", "beige", "More bottoms", fmt.Sprintf("%d tops share %d bottoms: one more pair makes many new outfits.", cnt["top"], cnt["bottom"]), 3)
	}
	if cnt["top"] > 0 && cnt["bottom"] >= cnt["top"]*2 {
		basic("tshirt", "white", "More tops", fmt.Sprintf("%d bottoms share %d tops.", cnt["bottom"], cnt["top"]), 3)
	}
	c.Style = ""
	if len(styles) > 0 {
		c.Style = styles[0]
	}
	for i := range cands {
		v := Virtual(cands[i].Kind, cands[i].Family, []string{cands[i].Style})
		pool := append(append([]Piece{}, all...), v)
		cc := c
		if cands[i].Style != "" {
			cc.Style = cands[i].Style
		}
		if cands[i].Category == "outerwear" && v.Warmth >= 4 {
			cc.TMax, cc.TMin, cc.Season = fp(6), fp(0), "winter"
		}
		if v.Rain {
			cc.Rain = true
		}
		n, ex := Count(pool, cc, 70, -1, 3)
		if v.Category == "accessory" || v.Category == "bag" {
			// not part of an outfit's core: the outfits it finishes
			n, ex = Count(all, cc, 70, 0, 2)
			for j := range ex {
				ex[j].Items = append(ex[j].Items, -1)
			}
			n /= 2
		}
		cands[i].Unlocks, cands[i].Examples = n, ex
		cands[i].Priority += float64(n) * 0.6
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Priority > cands[j].Priority })
	if len(cands) > limit {
		cands = cands[:limit]
	}
	for i := range cands {
		if cands[i].Examples == nil {
			cands[i].Examples = []Score{}
		}
	}
	return cands
}

func hasAvoid(avoid []string, fam string) bool {
	for _, a := range avoid {
		if a == fam {
			return true
		}
	}
	return false
}

func fp(v float64) *float64 { return &v }

// Verdict is "before you buy": would this be a good addition?
type Verdict struct {
	Verdict  string   `json:"verdict"` // great | good | think | skip
	Headline string   `json:"headline"`
	Reasons  []string `json:"reasons"`
	Similar  []int64  `json:"similar"`
	Outfits  int      `json:"outfits"`
	Examples []Score  `json:"examples"`
	Family   string   `json:"color_name"`
}

// Check works out how a piece someone is thinking of buying would fit in.
func Check(all []Piece, p Piece, c Context, styles []string) Verdict {
	p.ID = -1
	p.Ready = true
	p.DaysSince = -1
	v := Verdict{Reasons: []string{}, Similar: []int64{}, Examples: []Score{}, Family: p.Family()}
	sim := Similar(all, p)
	for _, s := range sim {
		v.Similar = append(v.Similar, s.ID)
	}
	if len(styles) > 0 {
		c.Style = styles[0]
	}
	pool := append(append([]Piece{}, all...), p)
	v.Outfits, v.Examples = Count(pool, c, 68, -1, 4)
	if v.Examples == nil {
		v.Examples = []Score{}
	}
	fam := p.Family()
	inStyle := false
	for _, sid := range styles {
		if st, ok := StyleByID(sid); ok && (has(st.Palette, fam) || IsNeutral(fam)) {
			inStyle = true
		}
	}
	works := 0
	for _, q := range all {
		if q.Category != p.Category && q.Category != "accessory" && q.Category != "bag" {
			r := Rate([]Piece{p, q}, c)
			if len(r.Warnings) == 0 || r.Score >= 70 {
				works++
			}
		}
	}
	switch {
	case len(sim) >= 2:
		v.Verdict, v.Headline = "skip", fmt.Sprintf("You already have %d like it", len(sim))
	case v.Outfits >= 8 && len(sim) == 0:
		v.Verdict, v.Headline = "great", fmt.Sprintf("A good buy: about %d new outfits", v.Outfits)
	case v.Outfits >= 3:
		v.Verdict, v.Headline = "good", fmt.Sprintf("It works: about %d outfits", v.Outfits)
		if len(sim) == 1 {
			v.Verdict, v.Headline = "think", "It works, but you have one much like it"
		}
	default:
		v.Verdict, v.Headline = "think", "Hard to wear with what you have"
	}
	if len(sim) > 0 {
		names := []string{}
		for _, s := range sim {
			names = append(names, s.label())
		}
		v.Reasons = append(v.Reasons, "Much like: "+strings.Join(names, ", "))
	}
	if IsNeutral(fam) {
		v.Reasons = append(v.Reasons, fam+" is a neutral: it goes with most of what you own")
	} else if !inStyle && len(styles) > 0 {
		v.Reasons = append(v.Reasons, fam+" is outside your style's colours")
	}
	v.Reasons = append(v.Reasons, fmt.Sprintf("Goes with %d of your pieces", works))
	if v.Outfits == 0 {
		v.Reasons = append(v.Reasons, "No complete outfit comes out of it yet")
	}
	return v
}

// SeasonOf is the season of a date in the northern hemisphere.
func SeasonOf(t time.Time) string {
	switch t.Month() {
	case 12, 1, 2:
		return "winter"
	case 3, 4, 5:
		return "spring"
	case 6, 7, 8:
		return "summer"
	}
	return "autumn"
}

// OccasionFor guesses what a calendar event needs to be dressed for, from its title.
func OccasionFor(title string) string {
	t := " " + strings.ToLower(title) + " "
	words := []struct {
		occ  string
		list []string
	}{
		{"wedding", []string{"wedding", "vjenčanj", "vjencanj", "svadb", "venčanj"}},
		{"formal", []string{"gala", "ceremony", "funeral", "dženaz", "dzenaz", "sahran", "opera", "ball ", "graduation", "matura", "promocij"}},
		{"interview", []string{"interview", "intervju", "razgovor za posao"}},
		{"formal_work", []string{"client", "board", "presentation", "pitch", "conference", "konferencij", "prezentacij", "klijent", "court"}},
		{"sport", []string{"gym", "workout", "training", "trening", "teretan", "yoga", "pilates", "run ", "running", "football", "fudbal", "nogomet", "basket", "tennis", "tenis", "swim", "plivanj", "crossfit", "fitness"}},
		{"outdoors", []string{"hike", "hiking", "planinar", "trail", "camping", "kamp", "picnic", "piknik", "walk", "šetnj", "setnj", "ski "}},
		{"travel", []string{"flight", "airport", "aerodrom", " let ", "train", "voz ", "bus to", "travel", "putovanj"}},
		{"date", []string{"date", "anniversary", "godišnjic", "godisnjic", "spoj"}},
		{"party", []string{"party", "birthday", "rođendan", "rodjendan", "proslav", "zabav", "drinks", "piće", "concert", "koncert", "club"}},
		{"dinner", []string{"dinner", "večer", "vecer", "restaurant", "restoran", "lunch", "ručak", "rucak"}},
		{"work", []string{"meeting", "sastan", "standup", "stand-up", "1:1", "office", "kancelarij", "ured", "work", "posao", "review", "sync", "workshop", "radionic"}},
	}
	for _, w := range words {
		for _, k := range w.list {
			if strings.Contains(t, k) {
				return w.occ
			}
		}
	}
	return ""
}

// Capsule picks a small set of pieces that make many outfits for a trip.
type CapsuleResult struct {
	Items   []int64  `json:"items"`
	Outfits int      `json:"outfits"`
	Extras  []string `json:"extras"`
	Notes   []string `json:"notes"`
}

// Capsule chooses what to pack for days at temperatures for a purpose.
func Capsule(all []Piece, days int, c Context, purpose string) CapsuleResult {
	if days < 1 {
		days = 1
	}
	if days > 30 {
		days = 30
	}
	switch purpose {
	case "business":
		c.Occasion = "work"
	case "wedding":
		c.Occasion = "wedding"
	case "outdoor", "ski":
		c.Occasion = "outdoors"
	case "beach":
		if c.TMax == nil {
			c.TMax, c.TMin = fp(29), fp(21)
		}
	}
	t, _ := c.Feel()
	want := map[string]int{
		"top":    int(math.Min(7, math.Ceil(float64(days)*0.7)+1)),
		"bottom": int(math.Min(3, math.Ceil(float64(days)/3))),
		"shoes":  2,
	}
	if days <= 2 {
		want["shoes"] = 1
	}
	if t < 18 {
		want["knit"] = 1
		if days > 4 && t < 12 {
			want["knit"] = 2
		}
	}
	if t < 17 || c.Rain {
		want["outerwear"] = 1
	}
	if purpose == "wedding" || purpose == "beach" || purpose == "holiday" {
		want["dress"] = 1
	}
	var picked []Piece
	pickedIDs := []int64{}
	cats := []string{"bottom", "shoes", "outerwear", "top", "knit", "dress"}
	for _, cat := range cats {
		for n := 0; n < want[cat]; n++ {
			var bestP *Piece
			bestS := -1e9
			for i, p := range all {
				if p.Category != cat || !p.Ready || hasID(pickedIDs, p.ID) {
					continue
				}
				s := suit(p, c) * 2
				// pairs well with what is packed already
				for _, q := range picked {
					if q.Category != p.Category {
						s += Rate([]Piece{p, q}, c).Score / 25
					}
				}
				for _, q := range picked {
					if q.Category == p.Category && closeColor(q.Family(), p.Family()) {
						s -= 3 // variety
					}
				}
				if IsNeutral(p.Family()) {
					s += 1.5
				}
				if s > bestS {
					bestS, bestP = s, &all[i]
				}
			}
			if bestP != nil {
				picked = append(picked, *bestP)
				pickedIDs = append(pickedIDs, bestP.ID)
			}
		}
	}
	out := CapsuleResult{Items: pickedIDs, Extras: []string{}, Notes: []string{}}
	n, _ := Count(picked, c, 62, 0, 0)
	out.Outfits = n
	nights := days
	out.Extras = append(out.Extras, fmt.Sprintf("Underwear × %d", nights+1), fmt.Sprintf("Socks × %d", nights+1), "Pyjamas")
	if t >= 24 || purpose == "beach" {
		out.Extras = append(out.Extras, "Swimwear", "Sunglasses", "Sun hat")
	}
	if t < 4 {
		out.Extras = append(out.Extras, "Gloves", "Warm hat", "Scarf")
	}
	if c.Rain {
		out.Extras = append(out.Extras, "Umbrella")
	}
	if purpose == "business" {
		out.Extras = append(out.Extras, "Belt", "Laptop bag")
	}
	if purpose == "outdoor" || purpose == "ski" {
		out.Extras = append(out.Extras, "Thermal base layer", "Hiking socks")
	}
	if purpose == "ski" {
		out.Extras = append(out.Extras, "Ski jacket and trousers", "Goggles", "Neck warmer")
	}
	missing := []string{}
	for _, cat := range cats {
		got := 0
		for _, p := range picked {
			if p.Category == cat {
				got++
			}
		}
		if got < want[cat] {
			for _, cc := range Categories {
				if cc.ID == cat {
					missing = append(missing, strings.ToLower(cc.Plural))
				}
			}
		}
	}
	if len(missing) > 0 {
		out.Notes = append(out.Notes, "Not enough "+joinWords(missing)+" in your wardrobe for this trip")
	}
	return out
}
