package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/style"
)

// dayWeather is one day's forecast where the person is (or where a trip goes).
type dayWeather struct {
	Date       string        `json:"date"`
	Place      string        `json:"place,omitempty"`
	Code       int           `json:"code"`
	Condition  string        `json:"condition"`
	Text       string        `json:"text,omitempty"`
	TMax       *float64      `json:"t_max"`
	TMin       *float64      `json:"t_min"`
	Precip     *float64      `json:"precip"`
	PrecipProb *float64      `json:"precip_prob"`
	Wind       *float64      `json:"wind"`
	Gust       *float64      `json:"gust"`
	UV         *float64      `json:"uv"`
	Sunrise    string        `json:"sunrise"`
	Sunset     string        `json:"sunset"`
	Hours      []hourWeather `json:"hours,omitempty"`
	Elevation  *float64      `json:"elevation,omitempty"`
	Source     string        `json:"source"` // weather (Pocket Weather) | open-meteo | demo
	URL        string        `json:"url,omitempty"`
	FetchedAt  string        `json:"fetched_at"`
}

type hourWeather struct {
	Time       string   `json:"time"` // HH:MM
	T          *float64 `json:"t"`
	PrecipProb *float64 `json:"precip_prob"`
	Code       int      `json:"code"`
	Gust       *float64 `json:"gust"`
}

// Rainy says whether it is worth dressing for rain.
func (w *dayWeather) Rainy() bool {
	if w == nil {
		return false
	}
	if w.PrecipProb != nil && *w.PrecipProb >= 50 {
		return true
	}
	if w.Precip != nil && *w.Precip >= 2 {
		return true
	}
	return (w.Code >= 51 && w.Code <= 67) || (w.Code >= 80 && w.Code <= 82) || w.Code >= 95
}

// Snowy says whether snow is expected.
func (w *dayWeather) Snowy() bool {
	return w != nil && ((w.Code >= 71 && w.Code <= 77) || w.Code == 85 || w.Code == 86)
}

// applyTo puts a day's weather into an outfit context.
func (w *dayWeather) applyTo(c *style.Context) {
	if w == nil {
		return
	}
	c.TMax, c.TMin, c.Rain = w.TMax, w.TMin, w.Rainy() || w.Snowy()
}

// short is the weather in a few words ("8–14°C, rain").
func (w *dayWeather) short() string {
	if w == nil {
		return ""
	}
	t := ""
	if w.TMin != nil && w.TMax != nil {
		t = fmt.Sprintf("%.0f–%.0f°C", *w.TMin, *w.TMax)
	} else if w.TMax != nil {
		t = fmt.Sprintf("%.0f°C", *w.TMax)
	}
	return joinNonEmpty(", ", t, strings.ToLower(w.Condition))
}

// forecastDays is how far ahead a forecast is worth asking for.
const forecastDays = 9

// forecastFor asks Pocket Weather (or, when it is not connected and the
// person allows it, Open-Meteo directly) for one day at one spot.
func (s *Server) forecastFor(ctx context.Context, c *clients, lat, lng float64, date string) (*dayWeather, string, error) {
	if s.cfg.DemoData {
		return demoWeather(date, lat), "", nil
	}
	sib, err := optional(c.conn(ctx, appWeather, scopeWeatherRead))
	if err != nil {
		return nil, "", err
	}
	if sib != nil {
		v, err := getJSONFrom(ctx, sib, "/api/v1/forecast", url.Values{"lat": {fmt.Sprint(lat)}, "lon": {fmt.Sprint(lng)}, "hours": {"240"}})
		if err != nil {
			return nil, "", fmt.Errorf("Pocket Weather: %w", err)
		}
		m, _ := v.(map[string]any)
		for _, day := range rows(m["days"]) {
			if str(day, "date") != date {
				continue
			}
			w := weatherFromDay(day)
			w.Date, w.Source, w.Elevation = date, "weather", fptr(m, "elevation")
			for _, h := range rows(m["hours"]) {
				t := str(h, "time")
				if !strings.HasPrefix(t, date) {
					continue
				}
				w.Hours = append(w.Hours, hourWeather{Time: hhmm(t), T: fptr(h, "temp"), PrecipProb: fptr(h, "precip_prob"), Code: int(num(h, "code")), Gust: fptr(h, "gust")})
			}
			return w, "", nil
		}
		return nil, "Pocket Weather has no forecast for that day yet", nil
	}
	if !s.online(c.st) {
		return nil, "connect Pocket Weather (or switch on looking things up online) for the forecast", nil
	}
	w, err := s.openMeteoDay(ctx, lat, lng, date)
	if err != nil {
		return nil, "", fmt.Errorf("the forecast: %w", err)
	}
	return w, "", nil
}

func weatherFromDay(day map[string]any) *dayWeather {
	return &dayWeather{Date: str(day, "date"), Code: int(num(day, "code")), Condition: str(day, "condition"), Text: str(day, "text"),
		TMax: fptr(day, "t_max"), TMin: fptr(day, "t_min"), Precip: fptr(day, "precip"), PrecipProb: fptr(day, "precip_prob"),
		Wind: fptr(day, "wind"), Gust: fptr(day, "gust"), UV: fptr(day, "uv"), Sunrise: hhmm(str(day, "sunrise")), Sunset: hhmm(str(day, "sunset"))}
}

// homeWeather is the weather for a day where the person lives: their
// first place in Pocket Weather, else the home town in Settings. The
// second value says why there is none.
func (s *Server) homeWeather(ctx context.Context, c *clients, date string) (*dayWeather, string) {
	key := fmt.Sprintf("hw:%d:%s", c.u.ID, date)
	if v, at, ok := s.store.CacheGet(ctx, key); ok {
		if t, err := time.Parse(time.RFC3339Nano, at); err == nil && time.Since(t) < 30*time.Minute {
			var w dayWeather
			if json.Unmarshal([]byte(v), &w) == nil {
				return &w, ""
			}
		}
	}
	w, why := s.homeWeatherNow(ctx, c, date)
	if w != nil {
		w.FetchedAt = time.Now().UTC().Format(time.RFC3339)
		_ = s.store.CachePut(ctx, key, toJSONs(w))
	}
	return w, why
}

func (s *Server) homeWeatherNow(ctx context.Context, c *clients, date string) (*dayWeather, string) {
	if s.cfg.DemoData {
		w := demoWeather(date, 43.86)
		w.Place = "Sarajevo"
		return w, ""
	}
	d := daysBetween(today(), date)
	if d < 0 || d > forecastDays {
		return nil, "no forecast that far ahead"
	}
	sib, err := optional(c.conn(ctx, appWeather, scopeWeatherRead))
	if err != nil {
		return nil, err.Error()
	}
	if sib != nil {
		cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		v, err := getJSONFrom(cctx, sib, "/api/v1/summary", nil)
		if err != nil {
			return nil, "Pocket Weather: " + err.Error()
		}
		m, _ := v.(map[string]any)
		place := obj(m, "place")
		if place == nil {
			return nil, firstNonEmpty(str(m, "text"), "add a place in Pocket Weather")
		}
		name := str(place, "name")
		pick := map[int]string{0: "today", 1: "tomorrow"}[d]
		if pick != "" && obj(m, pick) != nil {
			w := weatherFromDay(obj(m, pick))
			w.Date, w.Place, w.Source, w.URL = date, name, "weather", linkTo(sib, "/")
			if d == 0 {
				w.Text = str(m, "text")
			}
			return w, ""
		}
		lat, lng := num(place, "lat"), num(place, "lon")
		if lat == 0 && lng == 0 {
			lat, lng = num(place, "latitude"), num(place, "longitude")
		}
		w, why, err := s.forecastFor(ctx, c, lat, lng, date)
		if err != nil {
			return nil, err.Error()
		}
		if w != nil {
			w.Place, w.URL = name, linkTo(sib, "/")
		}
		return w, why
	}
	if c.st.HomeLat == nil || c.st.HomeLng == nil {
		return nil, "connect Pocket Weather, or set your home town in Settings → General"
	}
	if !s.online(c.st) {
		return nil, "looking things up online is off (Settings → General)"
	}
	w, err := s.openMeteoDay(ctx, *c.st.HomeLat, *c.st.HomeLng, date)
	if err != nil {
		return nil, "the forecast: " + err.Error()
	}
	w.Place = c.st.HomeName
	return w, ""
}

func fptr(m map[string]any, k string) *float64 {
	if m == nil {
		return nil
	}
	if v, ok := m[k].(float64); ok {
		return &v
	}
	return nil
}

// demoWeather is a believable day for DEMO_DATA (no network).
func demoWeather(date string, lat float64) *dayWeather {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		t = time.Now()
	}
	doy := float64(t.YearDay())
	base := 12 - 11*math.Cos((doy-15)/365*2*math.Pi) // ≈1°C mid-January … 23°C mid-July
	if lat < 40 {
		base += 7
	}
	wobble := float64((t.Day()*7)%9) - 4
	hi, lo := math.Round(base+5+wobble/2), math.Round(base-4+wobble/3)
	code := []int{1, 2, 3, 61, 1, 0, 80, 2, 3}[t.Day()%9]
	prob := map[bool]float64{true: 70, false: 15}[code >= 51]
	return &dayWeather{Date: date, Code: code, Condition: wmoWords(code), TMax: &hi, TMin: &lo, PrecipProb: &prob, Source: "demo", Sunrise: "06:58", Sunset: "16:32"}
}
