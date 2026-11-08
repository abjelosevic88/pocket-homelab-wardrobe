package server

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-wardrobe/internal/store"
)

// Pocket Core app links (docs/links.md in pocket-homelab-core), Wardrobe as
// the target: another Pocket app the person allowed in Core may act here for
// them with a pcx_ token, only within these scopes. Every route not listed
// in linkRoutes stays closed to it (403).

var linkOffers = []core.Offer{
	{Scope: "wardrobe:read", Name: "See your wardrobe", Description: "Read your clothes (what they are, their colours, how often you wore them), your saved outfits, what you wore and plan to wear on a day, today's suggestion for the weather, your packing lists, your style and the things worth buying — for your journal, the weather's \"what to wear\", a trip's packing, search and Pocket AI."},
	{Scope: "wardrobe:write", Name: "Log outfits and add clothes", Description: "Log what you wore or plan an outfit for a day, save an outfit, and add a piece of clothing (something you bought or that arrived). It may change or remove only what it added itself."},
}

type linkRoute struct {
	methods string
	path    *regexp.Regexp
	scope   string // "" = any linked app
}

var linkRoutes = []linkRoute{
	{"GET", regexp.MustCompile(`^/api/v1/auth/me$`), ""},
	{"GET", regexp.MustCompile(`^/api/v1/(summary|today|stats|items|outfits|wears|packs|ideas|styles|search|day/\d{4}-\d{2}-\d{2})$`), "wardrobe:read"},
	{"GET", regexp.MustCompile(`^/api/v1/(items|outfits|packs)/\d+$`), "wardrobe:read"},
	{"GET", regexp.MustCompile(`^/api/v1/photos/\d+$`), "wardrobe:read"},
	{"POST", regexp.MustCompile(`^/api/v1/suggest$`), "wardrobe:read"},
	{"POST", regexp.MustCompile(`^/api/v1/(items|outfits|wears)$`), "wardrobe:write"},
	{"PUT DELETE", regexp.MustCompile(`^/api/v1/(items|outfits|wears)/\d+$`), "wardrobe:write"},
}

var errLinkScope = errors.New("outside the link's scope")

// linkAllows reports whether a link token with these scopes may make this request.
func linkAllows(scopes []string, r *http.Request) bool {
	for _, lr := range linkRoutes {
		if !strings.Contains(" "+lr.methods+" ", " "+r.Method+" ") || !lr.path.MatchString(r.URL.Path) {
			continue
		}
		if lr.scope == "" {
			return true
		}
		for _, sc := range scopes {
			if sc == lr.scope {
				return true
			}
		}
	}
	return false
}

// linkUser validates a pcx_ token, checks the route against its scopes and
// maps it to the person it acts for. The second value is the acting app's slug.
func (s *Server) linkUser(ctx context.Context, tok string, r *http.Request) (*store.User, string, error) {
	in, err := s.core.Introspect(ctx, tok)
	if err != nil {
		return nil, "", err
	}
	if !in.Active || !in.Delegated() || in.Audience != "wardrobe" || in.User == nil || in.User.Email == "" || in.App == nil || in.App.Slug == "" {
		return nil, "", http.ErrNoCookie
	}
	if !linkAllows(strings.Fields(in.Scope), r) {
		return nil, "", errLinkScope
	}
	u, err := s.coreIdentityUser(ctx, in)
	return u, in.App.Slug, err
}

const linkAppKey ctxKey = 2

// linkApp is the acting app's slug when the request came in on a link token.
func linkApp(ctx context.Context) string {
	a, _ := ctx.Value(linkAppKey).(string)
	return a
}
