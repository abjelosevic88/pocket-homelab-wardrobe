package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/ai"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/notify"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/pocket"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/style"
)

// ---- settings ----

func mask(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "••••"
	}
	return s[:4] + "••••" + s[len(s)-4:]
}

// publicSettings is what one person sees: their settings with the secrets
// masked and, per Pocket app, the mode in effect (<app>_mode), whether it
// works and how (<app>_via: core | direct | "") and why not (<app>_detail).
func (s *Server) publicSettings(ctx context.Context, u *store.User, st store.Settings) map[string]any {
	out := map[string]any{
		"core_configured": s.core.Configured(), "core_registered": s.coreStatus()["registered"],
		"title": st.Title, "date_format": st.DateFormat, "time_format": st.TimeFormat, "week_start": st.WeekStart,
		"styles": st.Styles, "love_colors": st.LoveColors, "avoid_colors": st.AvoidColors, "presentation": st.Presentation, "color_season": st.ColorSeason, "color_notes": st.ColorNotes,
		"fit_notes": st.FitNotes, "size_top": st.SizeTop, "size_bottom": st.SizeBottom, "size_shoe": st.SizeShoe, "budget": st.Budget, "currency": st.Currency, "shops": st.Shops,
		"home_name": st.HomeName, "home_lat": st.HomeLat, "home_lng": st.HomeLng, "online": st.Online,
		"auto_laundry": st.AutoLaundry, "laundry_alert": st.LaundryAlert, "use_calendar": st.UseCalendar, "use_fitness": st.UseFitness, "ai_help": st.AIHelp, "ai_photos": st.AIPhotos,
		"notify_morning": st.NotifyMorning, "notify_hour": st.NotifyHour, "notify_evening": st.NotifyEvening, "evening_hour": st.EveningHour,
		"notify_url": st.NotifyURL, "notify_format": st.NotifyFormat, "notify_configured": st.NotifyURL != "",
		"ai_provider": st.AIProvider, "ai_pocket_model": st.AIPocketModel, "openai_api_key": mask(st.OpenAIKey), "openai_model": st.OpenAIModel, "openai_base_url": st.OpenAIBaseURL,
		"setup_complete": st.SetupComplete, "base_url": s.cfg.BaseURL, "demo": s.cfg.DemoData,
	}
	for _, a := range pocketApps {
		mode, via, detail := s.connStatus(ctx, u, st, a)
		out[a.slug+"_mode"], out[a.slug+"_via"], out[a.slug+"_detail"] = mode, via, detail
		out[a.slug+"_configured"] = via != ""
		_, url, token, public := connSettings(st, a)
		out[a.slug+"_url"], out[a.slug+"_token"], out[a.slug+"_public_url"] = url, mask(token), public
		if public == "" && via != "" {
			public = s.resolvePublic(ctx, a.slug)
		}
		out[a.slug+"_open_url"] = public
	}
	return out
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	st, err := s.store.GetSettings(r.Context(), actorID(r.Context()))
	if err != nil {
		s.fail(w, err, "settings")
		return
	}
	writeJSON(w, http.StatusOK, s.publicSettings(r.Context(), userFrom(r.Context()), st))
}

// handlePutSettings saves the caller's personal settings. The box-wide title
// is only taken from an admin.
func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r.Context())
	isAdmin := me.Role == "admin"
	cur, err := s.store.GetSettings(r.Context(), me.ID)
	if err != nil {
		s.fail(w, err, "settings")
		return
	}
	raw := map[string]any{}
	var in store.Settings
	keys, err := decodeKeys(r, &raw)
	if err != nil || remarshal(raw, &in) != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	bad := func(msg string) { writeErr(w, http.StatusBadRequest, msg) }
	if keys["title"] && isAdmin {
		cur.Title = strings.TrimSpace(in.Title)
		if cur.Title == "" {
			cur.Title = store.DefaultTitle
		}
	}
	if keys["date_format"] {
		cur.DateFormat = in.DateFormat
	}
	if keys["time_format"] {
		cur.TimeFormat = in.TimeFormat
	}
	if keys["week_start"] && (in.WeekStart == 1 || in.WeekStart == 7 || in.WeekStart == 6) {
		cur.WeekStart = in.WeekStart
	}
	if keys["notify_hour"] {
		if in.NotifyHour < 0 || in.NotifyHour > 23 {
			bad("the hour is 0 to 23")
			return
		}
		cur.NotifyHour = in.NotifyHour
	}
	secret := func(k string, dst *string, v string) {
		if keys[k] && !strings.Contains(v, "••") {
			*dst = strings.TrimSpace(v)
		}
	}
	url := func(k string, dst *string, v string) {
		if keys[k] {
			*dst = strings.TrimRight(strings.TrimSpace(v), "/")
		}
	}
	text := func(k string, dst *string, v string) {
		if keys[k] {
			*dst = strings.TrimSpace(v)
		}
	}
	boolIn := func(k string, dst *bool, v bool) {
		if keys[k] {
			*dst = v
		}
	}
	num := func(k string, dst *int, v, lo, hi int, msg string) bool {
		if !keys[k] {
			return true
		}
		if v < lo || v > hi {
			bad(msg)
			return false
		}
		*dst = v
		return true
	}
	families := func(list []string) []string {
		out := []string{}
		for _, f := range list {
			if fam, ok := style.FamilyByName(f); ok && !oneOf(fam.Name, out) {
				out = append(out, fam.Name)
			}
		}
		return out
	}
	if keys["styles"] {
		cur.Styles = []string{}
		for _, v := range in.Styles {
			if _, ok := style.StyleByID(v); ok && !oneOf(v, cur.Styles) && len(cur.Styles) < 4 {
				cur.Styles = append(cur.Styles, v)
			}
		}
	}
	if keys["love_colors"] {
		cur.LoveColors = families(in.LoveColors)
	}
	if keys["avoid_colors"] {
		cur.AvoidColors = families(in.AvoidColors)
	}
	if keys["presentation"] {
		if !oneOf(in.Presentation, []string{"any", "menswear", "womenswear"}) {
			bad("what you wear is menswear, womenswear or both")
			return
		}
		cur.Presentation = in.Presentation
	}
	if keys["color_season"] {
		if in.ColorSeason != "" && !oneOf(in.ColorSeason, style.Seasons) {
			bad("unknown colour season")
			return
		}
		cur.ColorSeason = in.ColorSeason
	}
	if keys["currency"] {
		if c := cleanCurrency(in.Currency); c != "" {
			cur.Currency = c
		}
	}
	if keys["budget"] {
		if in.Budget < 0 {
			bad("the budget cannot be negative")
			return
		}
		cur.Budget = in.Budget
	}
	if keys["shops"] {
		cur.Shops = []store.Shop{}
		for _, sh := range in.Shops {
			sh.Name, sh.URL = clip(strings.TrimSpace(sh.Name), 60), strings.TrimSpace(sh.URL)
			if sh.Name == "" || !strings.Contains(sh.URL, "{q}") || !(strings.HasPrefix(sh.URL, "https://") || strings.HasPrefix(sh.URL, "http://")) {
				bad("each shop needs a name and a search address with {q} where the words go")
				return
			}
			if len(cur.Shops) < 10 {
				cur.Shops = append(cur.Shops, sh)
			}
		}
	}
	if keys["home_lat"] || keys["home_lng"] {
		if in.HomeLat == nil || in.HomeLng == nil {
			cur.HomeLat, cur.HomeLng = nil, nil
		} else if *in.HomeLat < -90 || *in.HomeLat > 90 || *in.HomeLng < -180 || *in.HomeLng > 180 {
			bad("that place is off the map")
			return
		} else {
			cur.HomeLat, cur.HomeLng = in.HomeLat, in.HomeLng
		}
	}
	text("home_name", &cur.HomeName, in.HomeName)
	text("color_notes", &cur.ColorNotes, in.ColorNotes)
	text("fit_notes", &cur.FitNotes, in.FitNotes)
	text("size_top", &cur.SizeTop, in.SizeTop)
	text("size_bottom", &cur.SizeBottom, in.SizeBottom)
	text("size_shoe", &cur.SizeShoe, in.SizeShoe)
	if !num("laundry_alert", &cur.LaundryAlert, in.LaundryAlert, 0, 200, "the laundry alert is 0 to 200 pieces") ||
		!num("evening_hour", &cur.EveningHour, in.EveningHour, 0, 23, "the hour is 0 to 23") {
		return
	}
	boolIn("online", &cur.Online, in.Online)
	boolIn("auto_laundry", &cur.AutoLaundry, in.AutoLaundry)
	boolIn("use_calendar", &cur.UseCalendar, in.UseCalendar)
	boolIn("use_fitness", &cur.UseFitness, in.UseFitness)
	boolIn("ai_help", &cur.AIHelp, in.AIHelp)
	boolIn("ai_photos", &cur.AIPhotos, in.AIPhotos)
	boolIn("notify_evening", &cur.NotifyEvening, in.NotifyEvening)
	boolIn("notify_morning", &cur.NotifyMorning, in.NotifyMorning)
	text("notify_url", &cur.NotifyURL, in.NotifyURL)
	if keys["notify_format"] && in.NotifyFormat != "" {
		cur.NotifyFormat = in.NotifyFormat
	}
	// each Pocket app: mode, URL, token, public URL
	inMap := map[string]string{}
	for k, v := range raw {
		if sv, ok := v.(string); ok {
			inMap[k] = sv
		}
	}
	for _, a := range pocketApps {
		c := cur.Conns[a.slug]
		if v := inMap[a.slug+"_mode"]; keys[a.slug+"_mode"] && (v == modeCore || v == modeDirect || v == modeOff) {
			c.Mode = v
		}
		url(a.slug+"_url", &c.URL, inMap[a.slug+"_url"])
		secret(a.slug+"_token", &c.Token, inMap[a.slug+"_token"])
		url(a.slug+"_public_url", &c.PublicURL, inMap[a.slug+"_public_url"])
		if c != (store.Conn{}) {
			cur.Conns[a.slug] = c
		}
	}
	if keys["ai_provider"] {
		if !validAIProvider(in.AIProvider) {
			bad("AI is automatic, Pocket AI or OpenAI")
			return
		}
		cur.AIProvider = in.AIProvider
	}
	if keys["ai_pocket_model"] {
		if !validPocketModel(in.AIPocketModel) {
			bad("unknown model")
			return
		}
		cur.AIPocketModel = in.AIPocketModel
	}
	secret("openai_api_key", &cur.OpenAIKey, in.OpenAIKey)
	text("openai_model", &cur.OpenAIModel, in.OpenAIModel)
	url("openai_base_url", &cur.OpenAIBaseURL, in.OpenAIBaseURL)
	if err := s.store.SaveUserSettings(r.Context(), me.ID, cur); err != nil {
		s.fail(w, err, "save settings")
		return
	}
	if isAdmin && keys["title"] {
		if err := s.store.SaveBoxSettings(r.Context(), cur); err != nil {
			s.fail(w, err, "save settings")
			return
		}
	}
	s.forgetStatus(me.ID)
	for k := range keys {
		if isModeKey(k) {
			s.reportModesSoon()
			break
		}
	}
	writeJSON(w, http.StatusOK, s.publicSettings(r.Context(), me, cur))
}

// handleIntegrationTest checks a connection with the caller's own saved
// settings (or the URL/key typed into the form).
func (s *Server) handleIntegrationTest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Which  string `json:"which"`
		URL    string `json:"url"`
		Key    string `json:"key"`
		Format string `json:"format"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	uid := actorID(r.Context())
	st, _ := s.store.GetSettings(r.Context(), uid)
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	keyOr := func(saved string) string {
		if strings.Contains(in.Key, "••") || in.Key == "" {
			return saved
		}
		return in.Key
	}
	urlOr := func(saved string) string {
		if strings.TrimSpace(in.URL) == "" {
			return saved
		}
		return strings.TrimRight(strings.TrimSpace(in.URL), "/")
	}
	var msg string
	var err error
	if a, ok := appBySlug(in.Which); ok {
		mode, savedURL, savedTok, _ := connSettings(st, a)
		typed := in.Key != "" && !strings.Contains(in.Key, "••")
		var conn *siblingConn
		if tok := keyOr(savedTok); tok != "" && (typed || s.connMode(mode, savedTok) == modeDirect) {
			base := urlOr(savedURL)
			if base == "" {
				base = s.resolveSibling(ctx, a.slug)
			}
			conn = &siblingConn{client: pocket.New(base, tok, 15*time.Second), via: modeDirect}
		} else {
			conn, err = s.clients(ctx, uid).conn(ctx, a, a.scopes...)
		}
		if err == nil {
			how := "direct connection"
			if conn.via == modeCore {
				how = "through Pocket Core"
			}
			var v string
			if v, err = conn.client.Health(ctx); err == nil {
				msg = fmt.Sprintf("%s %s, %s", a.name, v, how)
			}
		}
	} else if in.Which == "openai" {
		c := ai.New(urlOr(st.OpenAIBaseURL), keyOr(st.OpenAIKey), st.OpenAIModel, 20*time.Second)
		var ids []string
		if ids, err = c.Models(ctx); err == nil {
			msg = fmt.Sprintf("The key works (%d models)", len(ids))
		}
	} else if in.Which == "notify" {
		f := in.Format
		if f == "" {
			f = st.NotifyFormat
		}
		n := notify.New(urlOr(st.NotifyURL), f)
		err = n.Send(ctx, notify.Message{Title: "Pocket Wardrobe", Body: notifyTestBody, Priority: "info", Tags: []string{"memo"}})
		msg = "Test message sent via " + n.Format
	} else {
		writeErr(w, http.StatusBadRequest, "unknown integration")
		return
	}
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": msg})
}

// ---- system / backup / export ----

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	var size int64
	if fi, err := os.Stat(s.cfg.DBPath); err == nil {
		size = fi.Size()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version": Version, "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH,
		"core":      s.coreStatus(),
		"auth_mode": s.cfg.AuthMode, "data_dir": s.cfg.DataDir, "db_path": s.cfg.DBPath, "db_bytes": size,
		"uptime": time.Since(s.started).Round(time.Second).String(), "timezone": time.Now().Location().String(),
		"counts": s.store.CountsFor(r.Context(), actorID(r.Context())),
	})
}

// handleBackup downloads the whole database, an admin-only box operation.
func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	dir := filepath.Join(s.cfg.DataDir, "backups")
	name := fmt.Sprintf("pocket-wardrobe-%s.db", time.Now().Format("20060102-150405"))
	dst := filepath.Join(dir, name)
	if err := s.store.Backup(r.Context(), dst); err != nil {
		s.fail(w, err, "backup")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeFile(w, r, dst)
}
