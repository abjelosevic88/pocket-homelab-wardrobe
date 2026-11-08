package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
)

// ---- the free online services ----
//
// Place search for your home town (OpenStreetMap's Nominatim, one request a
// second, cached) and, when Pocket Weather is not connected, the forecast
// (Open-Meteo). Neither needs a key; each person can switch them off
// (Settings → General), and DEMO_DATA never calls out.

var errOffline = &userErr{"looking things up online is switched off (Settings → General)"}

func (s *Server) online(st store.Settings) bool { return st.Online && !s.cfg.DemoData }

func (s *Server) getOnline(ctx context.Context, u string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", s.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")
	res, err := s.httpc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 300))
		return fmt.Errorf("HTTP %d %s", res.StatusCode, strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(io.LimitReader(res.Body, 32<<20)).Decode(v)
}

// ---- place search ----

type foundPlace struct {
	Name   string    `json:"name"`
	Detail string    `json:"detail"`
	Lat    float64   `json:"lat"`
	Lng    float64   `json:"lng"`
	Kind   string    `json:"kind"`
	Ele    *int      `json:"ele,omitempty"`
	BBox   []float64 `json:"bbox,omitempty"`
}

func (s *Server) searchPlaces(ctx context.Context, q string) ([]foundPlace, error) {
	key := "nominatim:" + strings.ToLower(q)
	if v, at, ok := s.store.CacheGet(ctx, key); ok {
		if t, err := time.Parse(time.RFC3339Nano, at); err == nil && time.Since(t) < 30*24*time.Hour {
			var out []foundPlace
			if json.Unmarshal([]byte(v), &out) == nil {
				return out, nil
			}
		}
	}
	s.lookMu.Lock()
	if wait := time.Second - time.Since(s.lookAt); wait > 0 {
		time.Sleep(wait)
	}
	s.lookAt = time.Now()
	s.lookMu.Unlock()
	var raw []struct {
		Name        string            `json:"name"`
		DisplayName string            `json:"display_name"`
		Lat         string            `json:"lat"`
		Lon         string            `json:"lon"`
		Category    string            `json:"category"`
		Type        string            `json:"type"`
		BBox        []string          `json:"boundingbox"`
		Extra       map[string]string `json:"extratags"`
	}
	u := "https://nominatim.openstreetmap.org/search?format=jsonv2&extratags=1&limit=8&q=" + url.QueryEscape(q)
	if err := s.getOnline(ctx, u, &raw); err != nil {
		return nil, err
	}
	out := []foundPlace{}
	for _, x := range raw {
		lat, _ := strconv.ParseFloat(x.Lat, 64)
		lng, _ := strconv.ParseFloat(x.Lon, 64)
		p := foundPlace{Name: firstNonEmpty(x.Name, strings.SplitN(x.DisplayName, ",", 2)[0]), Detail: x.DisplayName, Lat: lat, Lng: lng, Kind: x.Type}
		if e, err := strconv.Atoi(strings.TrimSuffix(x.Extra["ele"], " m")); err == nil {
			p.Ele = &e
		}
		if len(x.BBox) == 4 {
			for _, b := range x.BBox {
				f, _ := strconv.ParseFloat(b, 64)
				p.BBox = append(p.BBox, f)
			}
		}
		out = append(out, p)
	}
	_ = s.store.CachePut(ctx, key, toJSONs(out))
	return out, nil
}

// ---- heights ----

// fillHeights gives every point of a track a height from Open-Meteo's
// elevation model (90 m grid; good for the climb of a walk, not for a
// cliff edge). Points that have one keep it.
// ---- the forecast, when Pocket Weather is not connected ----

func (s *Server) openMeteoDay(ctx context.Context, lat, lng float64, date string) (*dayWeather, error) {
	key := fmt.Sprintf("om:%.2f,%.2f:%s", lat, lng, date)
	if v, at, ok := s.store.CacheGet(ctx, key); ok {
		if t, err := time.Parse(time.RFC3339Nano, at); err == nil && time.Since(t) < time.Hour {
			var w dayWeather
			if json.Unmarshal([]byte(v), &w) == nil {
				return &w, nil
			}
		}
	}
	u := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&timezone=auto&start_date=%s&end_date=%s"+
		"&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_sum,precipitation_probability_max,wind_speed_10m_max,wind_gusts_10m_max,uv_index_max,sunrise,sunset"+
		"&hourly=temperature_2m,precipitation_probability,weather_code,wind_gusts_10m", lat, lng, date, date)
	var out struct {
		Elevation float64 `json:"elevation"`
		Daily     struct {
			Code []int      `json:"weather_code"`
			TMax []*float64 `json:"temperature_2m_max"`
			TMin []*float64 `json:"temperature_2m_min"`
			Rain []*float64 `json:"precipitation_sum"`
			Prob []*float64 `json:"precipitation_probability_max"`
			Wind []*float64 `json:"wind_speed_10m_max"`
			Gust []*float64 `json:"wind_gusts_10m_max"`
			UV   []*float64 `json:"uv_index_max"`
			Rise []string   `json:"sunrise"`
			Set  []string   `json:"sunset"`
		} `json:"daily"`
		Hourly struct {
			Time []string   `json:"time"`
			T    []*float64 `json:"temperature_2m"`
			Prob []*float64 `json:"precipitation_probability"`
			Code []int      `json:"weather_code"`
			Gust []*float64 `json:"wind_gusts_10m"`
		} `json:"hourly"`
	}
	if err := s.getOnline(ctx, u, &out); err != nil {
		return nil, err
	}
	if len(out.Daily.Code) == 0 {
		return nil, errors.New("no forecast for that day yet")
	}
	at := func(a []*float64) *float64 {
		if len(a) > 0 {
			return a[0]
		}
		return nil
	}
	w := &dayWeather{Date: date, Code: out.Daily.Code[0], TMax: at(out.Daily.TMax), TMin: at(out.Daily.TMin), Precip: at(out.Daily.Rain), PrecipProb: at(out.Daily.Prob),
		Wind: at(out.Daily.Wind), Gust: at(out.Daily.Gust), UV: at(out.Daily.UV), Source: "open-meteo", Elevation: &out.Elevation}
	w.Condition = wmoWords(w.Code)
	if len(out.Daily.Rise) > 0 {
		w.Sunrise, w.Sunset = hhmm(out.Daily.Rise[0]), hhmm(out.Daily.Set[0])
	}
	for i, t := range out.Hourly.Time {
		h := hourWeather{Time: hhmm(t)}
		if i < len(out.Hourly.T) {
			h.T = out.Hourly.T[i]
		}
		if i < len(out.Hourly.Prob) {
			h.PrecipProb = out.Hourly.Prob[i]
		}
		if i < len(out.Hourly.Code) {
			h.Code = out.Hourly.Code[i]
		}
		if i < len(out.Hourly.Gust) {
			h.Gust = out.Hourly.Gust[i]
		}
		w.Hours = append(w.Hours, h)
	}
	_ = s.store.CachePut(ctx, key, toJSONs(w))
	return w, nil
}

// hhmm is the time part of "2026-11-07T07:05" (or of an RFC 3339 time).
func hhmm(v string) string {
	if i := strings.IndexByte(v, 'T'); i >= 0 && len(v) >= i+6 {
		return v[i+1 : i+6]
	}
	return v
}

// wmoWords is a WMO weather code in words.
func wmoWords(c int) string {
	switch {
	case c == 0:
		return "Clear"
	case c <= 2:
		return "Partly cloudy"
	case c == 3:
		return "Overcast"
	case c == 45 || c == 48:
		return "Fog"
	case c >= 51 && c <= 57:
		return "Drizzle"
	case c >= 61 && c <= 67:
		return "Rain"
	case c >= 71 && c <= 77:
		return "Snow"
	case c >= 80 && c <= 82:
		return "Showers"
	case c == 85 || c == 86:
		return "Snow showers"
	case c >= 95:
		return "Thunderstorms"
	}
	return "Mixed"
}

func toJSONs(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// ---- shop pages ----

var (
	reMetaTag = regexp.MustCompile(`(?is)<meta\s+[^>]*>`)
	reAttr    = regexp.MustCompile(`(?i)(property|name|content|itemprop)\s*=\s*("([^"]*)"|'([^']*)')`)
	reTitle   = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	rePrice   = regexp.MustCompile(`(\d{1,5}(?:[.,]\d{2})?)`)
)

// readProductPage reads a shop page's name, picture and price from its
// Open Graph / product tags (no JavaScript, so some shops say little).
func (s *Server) readProductPage(ctx context.Context, link string) (map[string]any, error) {
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, &userErr{"that does not look like a web address"}
	}
	if host := strings.ToLower(u.Hostname()); host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || net.ParseIP(host) != nil {
		return nil, &userErr{"only shop pages on the internet can be read"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; PocketWardrobe/1.0)")
	req.Header.Set("Accept", "text/html")
	res, err := s.httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("the shop answered HTTP %d", res.StatusCode)
	}
	b, _ := io.ReadAll(io.LimitReader(res.Body, 3<<20))
	page := string(b)
	meta := map[string]string{}
	for _, tag := range reMetaTag.FindAllString(page, 400) {
		var key, val string
		for _, a := range reAttr.FindAllStringSubmatch(tag, -1) {
			v := a[3] + a[4]
			switch strings.ToLower(a[1]) {
			case "property", "name", "itemprop":
				key = strings.ToLower(v)
			case "content":
				val = v
			}
		}
		if key != "" && val != "" && meta[key] == "" {
			meta[key] = html.UnescapeString(val)
		}
	}
	title := firstNonEmpty(meta["og:title"], meta["twitter:title"])
	if title == "" {
		if m := reTitle.FindStringSubmatch(page); m != nil {
			title = strings.TrimSpace(html.UnescapeString(m[1]))
		}
	}
	img := firstNonEmpty(meta["og:image"], meta["og:image:secure_url"], meta["twitter:image"], meta["image"])
	if img != "" {
		if iu, err := u.Parse(img); err == nil {
			img = iu.String()
		}
	}
	out := map[string]any{"title": clip(title, 200), "image": img, "shop": firstNonEmpty(meta["og:site_name"], strings.TrimPrefix(u.Hostname(), "www.")), "via": "page",
		"description": clip(firstNonEmpty(meta["og:description"], meta["description"]), 400)}
	if p := firstNonEmpty(meta["product:price:amount"], meta["og:price:amount"], meta["price"]); p != "" {
		if m := rePrice.FindString(p); m != "" {
			if f, err := strconv.ParseFloat(strings.Replace(m, ",", ".", 1), 64); err == nil {
				out["price"] = int64(math.Round(f * 100))
			}
		}
	}
	out["currency"] = strings.ToUpper(firstNonEmpty(meta["product:price:currency"], meta["og:price:currency"]))
	return out, nil
}

// handlePlaceSearch finds a town by name (for the weather at home).
func (s *Server) handlePlaceSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st := s.settingsOf(ctx, actorID(ctx))
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	if !s.online(st) {
		s.userFail(w, errOffline, "search")
		return
	}
	out, err := s.searchPlaces(ctx, clip(q, 200))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "place search: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
