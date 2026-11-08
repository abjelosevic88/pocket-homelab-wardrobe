package store

import (
	"encoding/json"
	"strings"
)

// User is an account on this box.
type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"` // admin | member
	CreatedAt    string `json:"created_at"`
}

// APIToken is a personal access token (hash stored, plaintext shown once).
type APIToken struct {
	ID         int64   `json:"id"`
	UserID     int64   `json:"user_id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	LastUsedAt *string `json:"last_used_at"`
	CreatedAt  string  `json:"created_at"`
}

// Color is one colour of a piece: its shade and the family it belongs to.
type Color struct {
	Hex  string `json:"hex"`
	Name string `json:"name"`
}

// Item is one piece in the wardrobe.
type Item struct {
	ID         int64    `json:"id"`
	UserID     int64    `json:"-"`
	Name       string   `json:"name"`
	Category   string   `json:"category"`
	Kind       string   `json:"kind"`
	Colors     []Color  `json:"colors"`
	Pattern    string   `json:"pattern"`
	Material   string   `json:"material"`
	Fit        string   `json:"fit"`
	Seasons    []string `json:"seasons"`
	Warmth     int      `json:"warmth"`
	Formality  int      `json:"formality"`
	Styles     []string `json:"styles"`
	Rain       bool     `json:"rain"`
	Brand      string   `json:"brand"`
	Size       string   `json:"size"`
	Price      *int64   `json:"price"`
	Currency   string   `json:"currency"`
	BoughtOn   string   `json:"bought_on"`
	Shop       string   `json:"shop"`
	Care       string   `json:"care"`
	Notes      string   `json:"notes"`
	Status     string   `json:"status"`
	StatusNote string   `json:"status_note"`
	StatusAt   string   `json:"status_at"`
	Favourite  bool     `json:"favourite"`
	Gone       string   `json:"gone"`
	GoneOn     string   `json:"gone_on"`
	SoldFor    *int64   `json:"sold_for"`
	PhotoID    *int64   `json:"photo_id"`
	Source     string   `json:"source"`
	SourceRef  string   `json:"source_ref"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`

	// worked out on read
	Wears    int    `json:"wears"`
	LastWorn string `json:"last_worn"`
	PerWear  *int64 `json:"per_wear"` // price ÷ wears
	PhotoURL string `json:"photo_url"`
	ThumbURL string `json:"thumb_url"`
}

// Outfit is a saved set of pieces.
type Outfit struct {
	ID        int64    `json:"id"`
	UserID    int64    `json:"-"`
	Name      string   `json:"name"`
	ItemIDs   []int64  `json:"item_ids"`
	Style     string   `json:"style"`
	Occasion  string   `json:"occasion"`
	Seasons   []string `json:"seasons"`
	Rating    int      `json:"rating"`
	Favourite bool     `json:"favourite"`
	Notes     string   `json:"notes"`
	PhotoID   *int64   `json:"photo_id"`
	MadeBy    string   `json:"made_by"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`

	Wears    int    `json:"wears"`
	LastWorn string `json:"last_worn"`
	PhotoURL string `json:"photo_url"`
}

// Wear is a day's outfit: worn, or planned for a day ahead.
type Wear struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"-"`
	Date      string          `json:"date"`
	Planned   bool            `json:"planned"`
	OutfitID  *int64          `json:"outfit_id"`
	ItemIDs   []int64         `json:"item_ids"`
	Occasion  string          `json:"occasion"`
	Event     string          `json:"event"`
	EventRef  string          `json:"event_ref"`
	Weather   json.RawMessage `json:"weather"`
	Feel      int             `json:"feel"`
	Notes     string          `json:"notes"`
	PhotoID   *int64          `json:"photo_id"`
	Source    string          `json:"source"`
	CreatedAt string          `json:"created_at"`

	PhotoURL string `json:"photo_url"`
}

// Idea is something worth buying.
type Idea struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"-"`
	Title     string `json:"title"`
	Category  string `json:"category"`
	Kind      string `json:"kind"`
	ColorHex  string `json:"color_hex"`
	ColorName string `json:"color_name"`
	Why       string `json:"why"`
	Style     string `json:"style"`
	Unlocks   int    `json:"unlocks"`
	PriceHint string `json:"price_hint"`
	URL       string `json:"url"`
	PhotoID   *int64 `json:"photo_id"`
	Status    string `json:"status"`
	MadeBy    string `json:"made_by"`
	WishRef   string `json:"wish_ref"`
	WishURL   string `json:"wish_url"`
	CreatedAt string `json:"created_at"`

	PhotoURL string `json:"photo_url"`
}

// PackItem is one piece on a packing list.
type PackItem struct {
	ItemID int64 `json:"item_id"`
	Qty    int   `json:"qty"`
	Packed bool  `json:"packed"`
}

// PackExtra is a line on a packing list that is not a piece ("Socks × 5").
type PackExtra struct {
	Text   string `json:"text"`
	Packed bool   `json:"packed"`
}

// Pack is packing for a trip.
type Pack struct {
	ID          int64           `json:"id"`
	UserID      int64           `json:"-"`
	Name        string          `json:"name"`
	Destination string          `json:"destination"`
	Lat         *float64        `json:"lat"`
	Lng         *float64        `json:"lng"`
	StartDate   string          `json:"start_date"`
	EndDate     string          `json:"end_date"`
	Purpose     string          `json:"purpose"`
	Items       []PackItem      `json:"items"`
	Extras      []PackExtra     `json:"extras"`
	Weather     json.RawMessage `json:"weather"`
	Notes       string          `json:"notes"`
	TripRef     string          `json:"trip_ref"`
	TripURL     string          `json:"trip_url"`
	CreatedAt   string          `json:"created_at"`
}

// Photo is a stored picture.
type Photo struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"-"`
	Kind      string `json:"kind"`
	Mime      string `json:"mime"`
	Data      []byte `json:"-"`
	Thumb     []byte `json:"-"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Bytes     int    `json:"bytes"`
	CreatedAt string `json:"created_at"`
}

// Values the API checks against.
var (
	ItemStates  = []string{"ready", "laundry", "repair", "lent", "stored"}
	GoneReasons = []string{"", "donated", "sold", "thrown", "lost"}
	Fits        = []string{"", "slim", "regular", "relaxed", "oversized"}
	IdeaStates  = []string{"idea", "wish", "bought", "dismissed"}
	Purposes    = []string{"holiday", "city", "business", "beach", "ski", "outdoor", "wedding", "other"}
)

// Conn is one person's connection to a sibling Pocket app.
type Conn struct {
	Mode      string `json:"mode,omitempty"`
	URL       string `json:"url,omitempty"`
	Token     string `json:"token,omitempty"`
	PublicURL string `json:"public_url,omitempty"`
}

// Shop is a place to look for something: a name and a search address
// with {q} where the words go.
type Shop struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// DefaultShops are where "look for it" searches at first.
var DefaultShops = []Shop{
	{"Google Shopping", "https://www.google.com/search?tbm=shop&q={q}"},
	{"Zalando", "https://www.zalando.com/catalog/?q={q}"},
	{"Vinted (second-hand)", "https://www.vinted.com/catalog?search_text={q}"},
}

// Settings are the box-wide values plus one person's preferences and their
// own connections.
type Settings struct {
	Title         string `json:"title"`
	SetupComplete bool   `json:"setup_complete"`

	DateFormat string `json:"date_format"`
	TimeFormat string `json:"time_format"`
	WeekStart  int    `json:"week_start"` // 1 = Monday, 7 = Sunday

	// Your style.
	Styles       []string `json:"styles"`       // the ways you like to dress (style ids)
	LoveColors   []string `json:"love_colors"`  // colour families
	AvoidColors  []string `json:"avoid_colors"` // colour families
	Presentation string   `json:"presentation"` // menswear | womenswear | any
	ColorSeason  string   `json:"color_season"` // from "find my colours": spring | summer | autumn | winter
	ColorNotes   string   `json:"color_notes"`
	FitNotes     string   `json:"fit_notes"` // what suits you, what you like to hide
	SizeTop      string   `json:"size_top"`
	SizeBottom   string   `json:"size_bottom"`
	SizeShoe     string   `json:"size_shoe"`
	Budget       int64    `json:"budget"` // a month, minor units, 0 = none
	Currency     string   `json:"currency"`
	Shops        []Shop   `json:"shops"`

	// Weather when Pocket Weather is not connected.
	HomeName string   `json:"home_name"`
	HomeLat  *float64 `json:"home_lat"`
	HomeLng  *float64 `json:"home_lng"`
	Online   bool     `json:"online"` // the forecast, place search and shop pages from the internet

	// Day to day.
	AutoLaundry  bool `json:"auto_laundry"`  // logging a wear puts tops and sportswear in the laundry
	LaundryAlert int  `json:"laundry_alert"` // tell me when this many pieces wait to be washed (0 = never)
	UseCalendar  bool `json:"use_calendar"`  // dress for today's events in Pocket Calendar
	UseFitness   bool `json:"use_fitness"`   // remind about gym clothes on a training day
	AIHelp       bool `json:"ai_help"`
	AIPhotos     bool `json:"ai_photos"` // photos may go to the AI (tagging, "rate my look")

	// Notifications.
	NotifyMorning bool   `json:"notify_morning"` // today's outfit
	NotifyHour    int    `json:"notify_hour"`
	NotifyEvening bool   `json:"notify_evening"` // tomorrow's plan and weather
	EveningHour   int    `json:"evening_hour"`
	NotifyURL     string `json:"notify_url"`
	NotifyFormat  string `json:"notify_format"` // apprise | ntfy | json

	// Each sibling Pocket app, by slug.
	Conns map[string]Conn `json:"conns"`

	// AI: Pocket AI through Core, or an OpenAI-compatible API with the
	// person's own key.
	AIProvider    string `json:"ai_provider"` // auto | pocket | openai
	AIPocketModel string `json:"ai_pocket_model"`
	OpenAIKey     string `json:"openai_api_key"`
	OpenAIModel   string `json:"openai_model"`
	OpenAIBaseURL string `json:"openai_base_url"`
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// CleanTags trims, de-duplicates and caps a list of tags.
func CleanTags(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range in {
		t = strings.TrimSpace(strings.TrimPrefix(t, "#"))
		k := strings.ToLower(t)
		if t == "" || seen[k] || len(out) >= 20 {
			continue
		}
		seen[k] = true
		out = append(out, t)
	}
	return out
}
