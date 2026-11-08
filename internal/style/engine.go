package style

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
)

// Piece is what the engine knows about one thing in the wardrobe.
type Piece struct {
	ID        int64
	Name      string
	Category  string
	Kind      string
	Colors    []string // hex, the main one first
	Pattern   string
	Warmth    int
	Formality int
	Styles    []string
	Seasons   []string
	Rain      bool
	Favourite bool
	DaysSince int // since last worn; -1 never
	Wears     int
	Ready     bool // clean and at home
}

// Family is the main colour family of a piece.
func (p Piece) Family() string {
	if len(p.Colors) == 0 {
		return "grey"
	}
	f := FamilyOf(p.Colors[0])
	if p.Kind == "jeans" || p.Kind == "denim_jacket" {
		switch f {
		case "blue", "light blue", "navy", "denim", "teal":
			return "denim"
		}
	}
	return f
}

func (p Piece) label() string {
	if p.Name != "" {
		return p.Name
	}
	if k, ok := KindByID(p.Kind); ok {
		return k.Label
	}
	return p.Category
}

// Context is what an outfit is for: the weather, the occasion, the style.
type Context struct {
	TMax     *float64 // the day's warmest (°C)
	TMin     *float64
	Rain     bool
	Season   string
	Occasion string
	Style    string   // the style to aim for ("" = any of Styles)
	Styles   []string // the person's styles
	Avoid    []string // colour families the person does not wear
	Love     []string // colour families the person likes
	Include  int64    // this piece must be in it
	Exclude  []int64
	Shuffle  int64 // a different seed gives other suggestions
	Indoor   bool  // the occasion keeps you inside (a coat is only for the way)
}

// Feel is the temperature to dress for: between the low and the high,
// nearer the high (most of the day is spent then).
func (c Context) Feel() (float64, bool) {
	switch {
	case c.TMax != nil && c.TMin != nil:
		return *c.TMin + (*c.TMax-*c.TMin)*0.65, true
	case c.TMax != nil:
		return *c.TMax - 2, true
	case c.TMin != nil:
		return *c.TMin + 4, true
	}
	switch c.Season {
	case "summer":
		return 26, false
	case "winter":
		return 3, false
	case "spring", "autumn":
		return 14, false
	}
	return 16, false
}

// Score is an outfit with how well it works and why.
type Score struct {
	Items    []int64  `json:"items"`
	Score    float64  `json:"score"`    // 0–100
	Reasons  []string `json:"reasons"`  // what works
	Warnings []string `json:"warnings"` // what does not
	Colors   []string `json:"colors"`   // the main families in it
	Warmth   float64  `json:"warmth"`
	Formal   float64  `json:"formality"`
}

type note struct {
	w    float64
	text string
}

// Rate scores a set of pieces as one outfit.
func Rate(ps []Piece, c Context) Score {
	var good, bad []note
	add := func(w float64, s string) {
		if w >= 0 {
			good = append(good, note{w, s})
		} else {
			bad = append(bad, note{-w, s})
		}
	}
	var top, knit, bottom, dress, outer, shoes []Piece
	var extras []Piece
	for _, p := range ps {
		switch p.Category {
		case "top", "active":
			if p.Category == "active" && strings.Contains(p.Kind, "bottom") {
				bottom = append(bottom, p)
			} else {
				top = append(top, p)
			}
		case "knit":
			knit = append(knit, p)
		case "bottom":
			bottom = append(bottom, p)
		case "dress":
			dress = append(dress, p)
		case "outerwear":
			outer = append(outer, p)
		case "shoes":
			shoes = append(shoes, p)
		default:
			extras = append(extras, p)
		}
	}
	score := 50.0
	// ---- complete? ----
	if len(dress) == 0 && (len(top)+len(knit) == 0 || len(bottom) == 0) {
		score -= 25
		if len(bottom) == 0 {
			add(-3, "needs something for the legs")
		} else {
			add(-3, "needs a top")
		}
	}
	if len(shoes) == 0 {
		score -= 8
		add(-1, "pick shoes")
	}
	if len(top) > 1 || len(bottom) > 1 || len(shoes) > 1 || len(dress) > 1 || len(outer) > 1 || len(knit) > 1 {
		score -= 15
		add(-3, "two of the same kind of piece")
	}
	main := append(append(append(append(append([]Piece{}, top...), knit...), bottom...), dress...), outer...)
	visible := append(append([]Piece{}, main...), shoes...)

	// ---- colour ----
	fams := []string{}
	seen := map[string]bool{}
	accents := []string{}
	for _, p := range visible {
		f := p.Family()
		if !seen[f] {
			seen[f] = true
			fams = append(fams, f)
			if !IsNeutral(f) {
				accents = append(accents, f)
			}
		}
	}
	st, hasStyle := StyleByID(c.Style)
	maxAcc := 1
	if hasStyle {
		maxAcc = st.Accents
	}
	switch n := len(accents); {
	case n == 0:
		if len(fams) == 1 && len(visible) > 2 {
			score += 4
			add(1.5, "one colour head to toe: a calm, tonal look")
		} else {
			score += 6
			add(1.2, "all neutrals: "+joinWords(fams)+" — easy and calm")
		}
		if hasStyle && st.Accents >= 2 {
			score -= 3
			add(-0.6, "no colour at all, for a bold style")
		}
	case n == 1:
		if IsEarth(accents[0]) {
			score += 8
			add(1.6, accents[0]+" with neutrals: warm and easy")
		} else {
			score += 9
			add(1.8, "one colour ("+accents[0]+") lifts the neutrals")
		}
		if hasStyle && st.Accents == 0 {
			score -= 5
			add(-1, accents[0]+" is more colour than a minimal look wants")
		}
	case n == 2:
		h1, s1, _, _ := HSL(HexOf(accents[0]))
		h2, s2, _, _ := HSL(HexOf(accents[1]))
		gap := hueGap(h1, h2)
		switch {
		case gap <= 40:
			score += 5
			add(1.2, accents[0]+" and "+accents[1]+" sit next to each other on the colour wheel")
		case gap >= 150 && (s1 < 0.55 || s2 < 0.55 || IsEarth(accents[0]) || IsEarth(accents[1])):
			score += 5
			add(1.2, accents[0]+" and "+accents[1]+" are opposites that balance")
		case gap >= 150:
			score += 1
			add(0.5, accents[0]+" and "+accents[1]+": a bold contrast")
		default:
			score -= 2
		}
		if maxAcc < 2 {
			score -= 6
			add(-1.2, "two colours ("+accents[0]+", "+accents[1]+"): calmer with one of them swapped for a neutral")
		}
	default:
		if maxAcc >= 3 {
			score += 2
			add(0.8, "colour blocking with "+joinWords(accents))
		} else {
			score -= 12
			add(-2, fmt.Sprintf("%d colours fight for attention (%s)", n, joinWords(accents)))
		}
	}
	// a few pairs people notice
	if seen["navy"] && seen["black"] && len(bottom)+len(top) > 0 {
		if hasStyle && (c.Style == "classic" || c.Style == "business") {
			score -= 4
			add(-0.8, "navy next to black can look like a mismatch")
		}
	}
	for _, f := range c.Avoid {
		if seen[f] {
			score -= 10
			add(-2, "has "+f+", a colour you said you avoid")
		}
	}
	for _, f := range c.Love {
		if seen[f] {
			score += 3
			add(0.7, "in "+f+", a colour you like")
			break
		}
	}
	// light against dark
	var upper, lower *Piece
	if len(top) > 0 {
		upper = &top[0]
	}
	if len(knit) > 0 {
		upper = &knit[0]
	}
	if len(bottom) > 0 {
		lower = &bottom[0]
	}
	if upper != nil && lower != nil && len(upper.Colors) > 0 && len(lower.Colors) > 0 {
		d := math.Abs(Lightness(upper.Colors[0]) - Lightness(lower.Colors[0]))
		if d >= 0.25 {
			score += 3
			add(0.6, "light against dark keeps it crisp")
		} else if d < 0.07 && upper.Family() != lower.Family() {
			score -= 3
			add(-0.6, "top and bottom are similar shades of different colours")
		}
	}

	// ---- patterns ----
	pat := 0
	for _, p := range main {
		if p.Pattern != "" && p.Pattern != "plain" {
			pat++
		}
	}
	switch {
	case pat == 1:
		score += 2
		add(0.5, "one pattern, the rest plain")
	case pat >= 2:
		if hasStyle && st.Patterns && pat == 2 {
			score -= 3
			add(-0.8, "two patterns: daring")
		} else {
			score -= 9
			add(-1.6, "two patterns compete; keep one")
		}
	}
	if hasStyle && !st.Patterns && pat > 0 {
		score -= 3
	}

	// ---- formality ----
	var fsum, fmin, fmax float64
	fmin = 9
	n := 0
	for _, p := range visible {
		f := float64(p.Formality)
		fsum += f
		n++
		fmin, fmax = math.Min(fmin, f), math.Max(fmax, f)
	}
	formal := 0.0
	if n > 0 {
		formal = fsum / float64(n)
	}
	occ := OccasionByID(c.Occasion)
	if n > 0 {
		gap := math.Abs(formal - occ.Formality)
		switch {
		case gap <= 0.5:
			score += 8
			add(1.4, "right for "+strings.ToLower(occ.Label))
		case gap <= 1:
			score += 3
		case formal < occ.Formality:
			score -= 6 * (gap - 0.5)
			add(-gap, "a bit casual for "+strings.ToLower(occ.Label))
		default:
			score -= 5 * (gap - 0.5)
			add(-gap*0.8, "dressier than "+strings.ToLower(occ.Label)+" needs")
		}
		if fmax-fmin >= 3 {
			score -= 7
			add(-1.2, "mixes very smart with very casual pieces")
		} else if fmax-fmin == 2 && (c.Style == "smart_casual") {
			score += 2
			add(0.5, "a smart piece with casual ones: smart casual done right")
		}
	}
	if occ.Active {
		active := 0
		for _, p := range visible {
			if p.Category == "active" || p.Kind == "running_shoes" || p.Kind == "leggings" || p.Kind == "joggers" || p.Kind == "sneakers" {
				active++
			}
		}
		if active >= 2 {
			score += 6
			add(1.2, "you can move in it")
		} else {
			score -= 10
			add(-2, "not made for sport")
		}
	}

	// ---- warmth ----
	t, known := c.Feel()
	warm := 0.0
	if len(dress) > 0 {
		warm += float64(dress[0].Warmth)
	} else if len(top) > 0 {
		warm += float64(top[0].Warmth)
	}
	if len(knit) > 0 {
		warm += float64(knit[0].Warmth) * 0.8
	}
	if len(outer) > 0 {
		warm += float64(outer[0].Warmth)
	}
	if len(bottom) > 0 {
		warm += (float64(bottom[0].Warmth) - 2) * 0.5
	}
	for _, p := range extras {
		if p.Kind == "scarf" || p.Kind == "beanie" || p.Kind == "gloves" {
			warm += 0.5
		}
	}
	lo, hi := warmthRange(t)
	tw := fmt.Sprintf("%.0f°C", t)
	if !known {
		tw = "the season"
	}
	switch {
	case warm >= lo && warm <= hi:
		score += 8
		if known {
			add(1.3, "warm enough for "+tw+" without overheating")
		} else {
			add(0.8, "right for "+tw)
		}
	case warm < lo:
		d := lo - warm
		score -= 7 * d
		if len(outer) == 0 && t < 15 {
			add(-d-0.5, "too cold for "+tw+" without a jacket")
		} else {
			add(-d, "light for "+tw+"; add a layer")
		}
	default:
		d := warm - hi
		if c.Indoor && len(outer) > 0 && warm-float64(outer[0].Warmth) <= hi {
			score -= 1
		} else {
			score -= 6 * d
			add(-d, "too warm for "+tw)
		}
	}
	if len(outer) > 0 && t >= 23 && outer[0].Kind != "blazer" {
		score -= 6
		add(-1, "no need for a jacket at "+tw)
	}
	if len(bottom) > 0 && bottom[0].Kind == "shorts" && t < 17 {
		score -= 8
		add(-1.5, "shorts at "+tw)
	}
	for _, p := range shoes {
		if p.Kind == "sandals" && t < 16 {
			score -= 8
			add(-1.5, "sandals at "+tw)
		}
		if p.Kind == "winter_boots" && t > 12 {
			score -= 5
			add(-1, "winter boots at "+tw)
		}
	}
	if c.Rain {
		dry := false
		for _, p := range append(append([]Piece{}, outer...), shoes...) {
			if p.Rain {
				dry = true
			}
		}
		if dry {
			score += 5
			add(1.2, "keeps the rain out")
		} else {
			score -= 2
			add(-0.4, "rain on the way: take an umbrella")
		}
	}

	// ---- season ----
	if c.Season != "" {
		off := 0
		for _, p := range main {
			if len(p.Seasons) > 0 && !has(p.Seasons, c.Season) {
				off++
			}
		}
		if off > 0 {
			score -= 3 * float64(off)
			add(-0.5*float64(off), "has a piece meant for another season")
		}
	}

	// ---- style ----
	if hasStyle {
		match, against := 0, 0
		for _, p := range visible {
			if has(p.Styles, st.ID) || has(st.Prefers, p.Kind) {
				match++
			}
			if has(st.Avoids, p.Kind) {
				against++
			}
			if !has(st.Palette, p.Family()) && !IsNeutral(p.Family()) {
				against++
			}
		}
		if len(visible) > 0 {
			share := float64(match) / float64(len(visible))
			score += 14*share - 5
			if share >= 0.6 {
				add(1.1+share, "very "+strings.ToLower(st.Name))
			}
		}
		if against > 0 {
			score -= 4 * float64(against)
			add(-0.7*float64(against), "a piece that is not quite "+strings.ToLower(st.Name))
		}
		if formal > 0 && (formal < float64(st.FormalLo)-0.5 || formal > float64(st.FormalHi)+0.5) {
			score -= 4
		}
	}

	// ---- wearing it ----
	for _, p := range visible {
		if !p.Ready {
			score -= 30
			add(-4, p.label()+" is not ready to wear")
		}
		if p.DaysSince == 0 {
			score -= 6
			add(-0.8, p.label()+" was worn today")
		} else if p.DaysSince == 1 {
			score -= 3
			add(-0.5, p.label()+" was worn yesterday")
		}
		if p.Favourite {
			score += 1.5
		}
	}
	never := 0
	for _, p := range visible {
		if p.DaysSince < 0 && p.Wears == 0 {
			never++
		}
	}
	if never > 0 {
		score += 2
		add(0.4, "wears something you have not worn yet")
	}

	sort.SliceStable(good, func(i, j int) bool { return good[i].w > good[j].w })
	sort.SliceStable(bad, func(i, j int) bool { return bad[i].w > bad[j].w })
	out := Score{Items: []int64{}, Score: math.Round(clamp(score, 0, 100)), Reasons: []string{}, Warnings: []string{}, Colors: fams, Warmth: math.Round(warm*10) / 10, Formal: math.Round(formal*10) / 10}
	for i, g := range good {
		if i < 3 {
			out.Reasons = append(out.Reasons, g.text)
		}
	}
	for i, b := range bad {
		if i < 3 {
			out.Warnings = append(out.Warnings, b.text)
		}
	}
	for _, p := range ps {
		out.Items = append(out.Items, p.ID)
	}
	return out
}

// warmthRange is how warm an outfit should be at a temperature.
func warmthRange(t float64) (lo, hi float64) {
	switch {
	case t >= 26:
		return 0.5, 2.2
	case t >= 20:
		return 1, 3.2
	case t >= 15:
		return 2, 4.6
	case t >= 9:
		return 3.6, 6.6
	case t >= 3:
		return 5.2, 8.6
	}
	return 6.8, 11
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func has(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func hasID(list []int64, v int64) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func joinWords(w []string) string {
	switch len(w) {
	case 0:
		return ""
	case 1:
		return w[0]
	}
	return strings.Join(w[:len(w)-1], ", ") + " and " + w[len(w)-1]
}

// suit is how well one piece fits the day on its own (to keep the search small).
func suit(p Piece, c Context) float64 {
	s := 0.0
	t, _ := c.Feel()
	k := p.Warmth
	switch p.Category {
	case "outerwear":
		need := 0
		switch {
		case t < 3:
			need = 5
		case t < 9:
			need = 4
		case t < 15:
			need = 3
		case t < 21:
			need = 2
		}
		s -= math.Abs(float64(k-need)) * 1.5
	case "knit":
		if t > 20 {
			s -= 4
		}
	case "top", "dress":
		if t > 22 && k >= 3 {
			s -= 2
		}
	}
	occ := OccasionByID(c.Occasion)
	s -= math.Abs(float64(p.Formality)-occ.Formality) * 1.2
	if st, ok := StyleByID(c.Style); ok {
		if has(p.Styles, st.ID) || has(st.Prefers, p.Kind) {
			s += 2
		}
		if has(st.Avoids, p.Kind) {
			s -= 2.5
		}
	}
	if c.Season != "" && len(p.Seasons) > 0 && !has(p.Seasons, c.Season) {
		s -= 2
	}
	if p.Favourite {
		s += 0.5
	}
	if p.DaysSince >= 0 && p.DaysSince <= 1 {
		s -= 1.5
	}
	return s
}

func best(ps []Piece, c Context, n int, rnd *rand.Rand) []Piece {
	type sp struct {
		p Piece
		s float64
	}
	var l []sp
	for _, p := range ps {
		l = append(l, sp{p, suit(p, c) + rnd.Float64()*1.2})
	}
	sort.SliceStable(l, func(i, j int) bool { return l[i].s > l[j].s })
	out := []Piece{}
	for i := 0; i < len(l) && i < n; i++ {
		out = append(out, l[i].p)
	}
	return out
}

// Generate suggests up to n different outfits from the pieces for a day.
func Generate(all []Piece, c Context, n int) []Score {
	rnd := rand.New(rand.NewSource(c.Shuffle*7919 + 17))
	by := map[string][]Piece{}
	var must *Piece
	for i, p := range all {
		if !p.Ready || hasID(c.Exclude, p.ID) {
			continue
		}
		if p.ID == c.Include {
			must = &all[i]
		}
		cat := p.Category
		if cat == "active" {
			if strings.Contains(p.Kind, "bottom") {
				cat = "bottom"
			} else {
				cat = "top"
			}
		}
		by[cat] = append(by[cat], p)
	}
	if must != nil {
		cat := must.Category
		if cat == "active" {
			cat = map[bool]string{true: "bottom", false: "top"}[strings.Contains(must.Kind, "bottom")]
		}
		by[cat] = []Piece{*must}
	}
	t, _ := c.Feel()
	tops, bottoms, dresses := best(by["top"], c, 14, rnd), best(by["bottom"], c, 10, rnd), best(by["dress"], c, 6, rnd)
	knits, outers, shoes := best(by["knit"], c, 6, rnd), best(by["outerwear"], c, 6, rnd), best(by["shoes"], c, 8, rnd)
	if must != nil && must.Category == "dress" {
		tops, bottoms = nil, nil
	}
	if must != nil && (must.Category == "top" || must.Category == "bottom") {
		dresses = nil
	}
	type cand struct {
		ps []Piece
		s  Score
	}
	beam := func(list []cand, k int) []cand {
		sort.SliceStable(list, func(i, j int) bool { return list[i].s.Score > list[j].s.Score })
		if len(list) > k {
			list = list[:k]
		}
		return list
	}
	var bases []cand
	for _, tp := range tops {
		for _, b := range bottoms {
			ps := []Piece{tp, b}
			bases = append(bases, cand{ps, Rate(ps, c)})
		}
	}
	for _, d := range dresses {
		ps := []Piece{d}
		bases = append(bases, cand{ps, Rate(ps, c)})
	}
	bases = beam(bases, 40)
	var withShoes []cand
	for _, b := range bases {
		if len(shoes) == 0 {
			withShoes = append(withShoes, b)
			continue
		}
		for _, sh := range shoes {
			ps := append(append([]Piece{}, b.ps...), sh)
			withShoes = append(withShoes, cand{ps, Rate(ps, c)})
		}
	}
	withShoes = beam(withShoes, 50)
	var layered []cand
	for _, b := range withShoes {
		layered = append(layered, b)
		if t < 19 {
			for _, k := range knits {
				ps := append(append([]Piece{}, b.ps...), k)
				layered = append(layered, cand{ps, Rate(ps, c)})
			}
		}
	}
	layered = beam(layered, 50)
	var full []cand
	for _, b := range layered {
		full = append(full, b)
		if t < 21 || c.Rain || c.Occasion == "formal_work" || c.Occasion == "interview" || c.Occasion == "wedding" {
			for _, o := range outers {
				ps := append(append([]Piece{}, b.ps...), o)
				full = append(full, cand{ps, Rate(ps, c)})
			}
		}
	}
	// a little noise so "show others" gives others
	for i := range full {
		full[i].s.Score += rnd.Float64() * 2.5
	}
	full = beam(full, 200)
	out := []Score{}
	used := map[int64]int{}
	for _, f := range full {
		if len(out) >= n {
			break
		}
		same := false
		for _, o := range out {
			if overlap(o.Items, f.s.Items) >= len(f.s.Items)-1 {
				same = true
				break
			}
		}
		if same {
			continue
		}
		reuse := 0
		for _, id := range f.s.Items {
			reuse += used[id]
		}
		if reuse > 2 && len(full) > n*3 {
			continue
		}
		f.s.Score = math.Round(math.Min(100, f.s.Score))
		out = append(out, f.s)
		for _, id := range f.s.Items {
			if id != c.Include {
				used[id]++
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

func overlap(a, b []int64) int {
	n := 0
	for _, x := range a {
		if hasID(b, x) {
			n++
		}
	}
	return n
}

// Count is how many good outfits the pieces make (score at least min),
// with a few of the best as examples. include (when not 0) must be in
// each; the tops, bottoms and shoes that suit the day most are tried in
// every combination, with the best layer for each.
func Count(all []Piece, c Context, min float64, include int64, examples int) (int, []Score) {
	rnd := rand.New(rand.NewSource(3))
	by := map[string][]Piece{}
	var must *Piece
	for i, p := range all {
		if !p.Ready {
			continue
		}
		if p.ID == include && include != 0 {
			must = &all[i]
		}
		cat := p.Category
		if cat == "active" {
			cat = map[bool]string{true: "bottom", false: "top"}[strings.Contains(p.Kind, "bottom")]
		}
		by[cat] = append(by[cat], p)
	}
	pick := func(cat string, n int) []Piece {
		if must != nil && (must.Category == cat || (must.Category == "active" && cat == map[bool]string{true: "bottom", false: "top"}[strings.Contains(must.Kind, "bottom")])) {
			return []Piece{*must}
		}
		return best(by[cat], c, n, rnd)
	}
	tops, bottoms, shoes := pick("top", 16), pick("bottom", 12), pick("shoes", 8)
	dresses, knits, outers := pick("dress", 8), pick("knit", 6), pick("outerwear", 6)
	if must != nil && must.Category == "dress" {
		tops, bottoms = nil, nil
	}
	if must != nil && (must.Category == "top" || must.Category == "bottom") {
		dresses = nil
	}
	t, _ := c.Feel()
	var bases [][]Piece
	for _, tp := range tops {
		for _, b := range bottoms {
			bases = append(bases, []Piece{tp, b})
		}
	}
	for _, d := range dresses {
		bases = append(bases, []Piece{d})
	}
	n := 0
	var good []Score
	for _, b := range bases {
		for _, sh := range append(shoes, Piece{ID: 0}) {
			ps := append([]Piece{}, b...)
			if sh.ID != 0 {
				ps = append(ps, sh)
			} else if len(shoes) > 0 {
				continue
			}
			// the best layer for the day, if any helps
			bestS := Rate(ps, c)
			bestPs := ps
			if t < 19 || must != nil && (must.Category == "knit" || must.Category == "outerwear") {
				for _, k := range append(knits, outers...) {
					q := append(append([]Piece{}, ps...), k)
					if r := Rate(q, c); r.Score > bestS.Score {
						bestS, bestPs = r, q
					}
				}
			}
			if must != nil {
				in := false
				for _, p := range bestPs {
					if p.ID == must.ID {
						in = true
					}
				}
				if !in {
					continue
				}
			}
			if bestS.Score >= min {
				n++
				good = append(good, bestS)
			}
		}
	}
	sort.SliceStable(good, func(i, j int) bool { return good[i].Score > good[j].Score })
	ex := []Score{}
	used := map[int64]int{}
	for _, g := range good {
		if len(ex) >= examples {
			break
		}
		reuse := 0
		for _, id := range g.Items {
			if id != include {
				reuse += used[id]
			}
		}
		if reuse > 1 {
			continue
		}
		ex = append(ex, g)
		for _, id := range g.Items {
			used[id]++
		}
	}
	return n, ex
}
