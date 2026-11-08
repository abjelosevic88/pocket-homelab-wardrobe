package style

import (
	"math"
	"strconv"
	"strings"
)

// Family is a named colour group ("navy", "camel") with a typical shade.
type Family struct {
	Name    string `json:"name"`
	Hex     string `json:"hex"`
	Neutral bool   `json:"neutral"`
	Earth   bool   `json:"earth,omitempty"` // muted: behaves almost like a neutral
}

// Families from dark to light neutrals, then the colours round the wheel.
var Families = []Family{
	{"black", "#1b1b1d", true, false},
	{"charcoal", "#3b3e44", true, false},
	{"grey", "#8b8e94", true, false},
	{"white", "#f6f5f1", true, false},
	{"cream", "#efe5cf", true, false},
	{"beige", "#d4c09c", true, false},
	{"camel", "#bf935f", true, false},
	{"brown", "#6a4630", true, false},
	{"navy", "#1f2b48", true, false},
	{"denim", "#4b6a8d", true, false},
	{"khaki", "#a89b6c", true, true},
	{"olive", "#5f6236", false, true},
	{"burgundy", "#6c2232", false, true},
	{"rust", "#a5512b", false, true},
	{"mustard", "#c8992c", false, true},
	{"red", "#c43a31", false, false},
	{"orange", "#e2792f", false, false},
	{"yellow", "#ecc94b", false, false},
	{"green", "#3d7f50", false, false},
	{"teal", "#21797a", false, false},
	{"light blue", "#a7c7e7", false, false},
	{"blue", "#2f62b0", false, false},
	{"purple", "#6b4692", false, false},
	{"lavender", "#bcaee0", false, false},
	{"pink", "#e58fb0", false, false},
	{"blush", "#ebc8c2", false, false},
	{"gold", "#c9a64a", true, false},
	{"silver", "#c0c3c8", true, false},
}

// FamilyByName finds a family.
func FamilyByName(name string) (Family, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, f := range Families {
		if f.Name == name {
			return f, true
		}
	}
	return Family{}, false
}

// ParseHex reads #rgb or #rrggbb.
func ParseHex(h string) (r, g, b float64, ok bool) {
	h = strings.TrimPrefix(strings.TrimSpace(h), "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return float64(v>>16&255) / 255, float64(v>>8&255) / 255, float64(v&255) / 255, true
}

// ValidHex reports whether h is a colour.
func ValidHex(h string) bool {
	_, _, _, ok := ParseHex(h)
	return ok && strings.HasPrefix(strings.TrimSpace(h), "#")
}

func lin(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// Lab is CIE L*a*b* (D65).
func Lab(hex string) (l, a, b float64, ok bool) {
	r, g, bl, ok := ParseHex(hex)
	if !ok {
		return 0, 0, 0, false
	}
	r, g, bl = lin(r), lin(g), lin(bl)
	x := (r*0.4124 + g*0.3576 + bl*0.1805) / 0.95047
	y := r*0.2126 + g*0.7152 + bl*0.0722
	z := (r*0.0193 + g*0.1192 + bl*0.9505) / 1.08883
	f := func(t float64) float64 {
		if t > 0.008856 {
			return math.Cbrt(t)
		}
		return 7.787*t + 16.0/116
	}
	fx, fy, fz := f(x), f(y), f(z)
	return 116*fy - 16, 500 * (fx - fy), 200 * (fy - fz), true
}

// HSL of a colour: hue 0–360, saturation and lightness 0–1.
func HSL(hex string) (h, s, l float64, ok bool) {
	r, g, b, ok := ParseHex(hex)
	if !ok {
		return 0, 0, 0, false
	}
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (mx + mn) / 2
	d := mx - mn
	if d == 0 {
		return 0, 0, l, true
	}
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	switch mx {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h * 60, s, l, true
}

// FamilyOf names the nearest colour family of a hex colour.
func FamilyOf(hex string) string {
	l1, a1, b1, ok := Lab(hex)
	if !ok {
		return ""
	}
	best, bd := "", math.MaxFloat64
	for _, f := range Families {
		if f.Name == "gold" || f.Name == "silver" {
			continue // only when named
		}
		l2, a2, b2, _ := Lab(f.Hex)
		// lightness counts less than hue: a pale navy is still navy
		d := math.Pow((l1-l2)*0.75, 2) + math.Pow(a1-a2, 2) + math.Pow(b1-b2, 2)
		if d < bd {
			best, bd = f.Name, d
		}
	}
	return best
}

// IsNeutral says whether a family goes with nearly everything.
func IsNeutral(family string) bool {
	f, ok := FamilyByName(family)
	return ok && f.Neutral
}

// IsEarth says whether a family is a muted colour.
func IsEarth(family string) bool {
	f, ok := FamilyByName(family)
	return ok && f.Earth
}

// HexOf is a family's typical shade.
func HexOf(family string) string {
	if f, ok := FamilyByName(family); ok {
		return f.Hex
	}
	return "#888888"
}

// hueGap is the distance between two hues on the colour wheel (0–180).
func hueGap(a, b float64) float64 {
	d := math.Abs(a - b)
	if d > 180 {
		d = 360 - d
	}
	return d
}

// Lightness of a colour, 0–1.
func Lightness(hex string) float64 {
	_, _, l, _ := HSL(hex)
	return l
}
