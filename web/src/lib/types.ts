export interface User { id: number; email: string; name: string; role: 'admin' | 'member'; created_at: string }
export interface APIToken { id: number; user_id: number; name: string; prefix: string; last_used_at: string | null; created_at: string }

export type ConnMode = 'core' | 'direct' | 'off'
export type ConnVia = '' | 'core' | 'direct'
export type AppSlug = 'weather' | 'calendar' | 'travel' | 'trails' | 'fitness' | 'wishlist' | 'money' | 'parcels' | 'tasks' | 'chat' | 'notes'

type ConnFields = { [K in AppSlug as `${K}_mode`]: ConnMode } & { [K in AppSlug as `${K}_via`]: ConnVia } & { [K in AppSlug as `${K}_detail` | `${K}_url` | `${K}_token` | `${K}_public_url` | `${K}_open_url`]: string } & { [K in AppSlug as `${K}_configured`]: boolean }

export interface Shop { name: string; url: string }

export type Settings = ConnFields & {
  core_configured: boolean
  core_registered?: boolean
  title: string; date_format: string; time_format: string; week_start: number
  styles: string[]; love_colors: string[]; avoid_colors: string[]; presentation: 'any' | 'menswear' | 'womenswear'
  color_season: string; color_notes: string; fit_notes: string; size_top: string; size_bottom: string; size_shoe: string
  budget: number; currency: string; shops: Shop[]
  home_name: string; home_lat: number | null; home_lng: number | null; online: boolean
  auto_laundry: boolean; laundry_alert: number; use_calendar: boolean; use_fitness: boolean; ai_help: boolean; ai_photos: boolean
  notify_morning: boolean; notify_hour: number; notify_evening: boolean; evening_hour: number
  notify_url: string; notify_format: string; notify_configured: boolean
  ai_provider: 'auto' | 'pocket' | 'openai'; ai_pocket_model: string; openai_api_key: string; openai_model: string; openai_base_url: string
  setup_complete: boolean; base_url: string; demo: boolean
}

export type AppStateName = 'on' | 'partly' | 'off' | 'link_off' | 'missing' | 'old' | 'no_core' | 'not_set_up' | 'error'
export interface AppState { slug: AppSlug; name: string; reason: string; mode: ConnMode; via: ConnVia; state: AppStateName; detail: string; fix: string; on: string[]; open_url: string }

export type Category = 'top' | 'knit' | 'bottom' | 'dress' | 'outerwear' | 'shoes' | 'bag' | 'accessory' | 'active' | 'other'
export type ItemStatus = 'ready' | 'laundry' | 'repair' | 'lent' | 'stored'

export interface Color { hex: string; name: string }

export interface Item {
  id: number; name: string; category: Category; kind: string; colors: Color[]; pattern: string; material: string; fit: string
  seasons: string[]; warmth: number; formality: number; styles: string[]; rain: boolean; brand: string; size: string
  price: number | null; currency: string; bought_on: string; shop: string; care: string; notes: string
  status: ItemStatus; status_note: string; status_at: string; favourite: boolean; gone: string; gone_on: string; sold_for: number | null
  photo_id: number | null; source: string; source_ref: string; created_at: string; updated_at: string
  wears: number; last_worn: string; per_wear: number | null; photo_url: string; thumb_url: string
}

export interface Outfit {
  id: number; name: string; item_ids: number[]; style: string; occasion: string; seasons: string[]; rating: number; favourite: boolean
  notes: string; photo_id: number | null; made_by: string; created_at: string; updated_at: string; wears: number; last_worn: string; photo_url: string
}

export interface Wear {
  id: number; date: string; planned: boolean; outfit_id: number | null; item_ids: number[]; occasion: string; event: string; event_ref: string
  weather: { t_max?: number; t_min?: number; condition?: string; code?: number; place?: string } | null; feel: number; notes: string; photo_id: number | null; source: string; created_at: string; photo_url: string
}

export interface Score { items: number[]; score: number; reasons: string[]; warnings: string[]; colors: string[]; warmth: number; formality: number }

export interface DayWeather {
  date: string; place?: string; code: number; condition: string; text?: string; t_max: number | null; t_min: number | null
  precip: number | null; precip_prob: number | null; wind: number | null; uv: number | null; sunrise: string; sunset: string; source: string; url?: string
}

export interface Occasion { id: string; label: string; formality: number; icon: string; active?: boolean }
export interface Kind { id: string; label: string; category: Category; warmth: number; formality: number; shape: string; rain?: boolean }
export interface StyleDef { id: string; name: string; description: string; palette: string[]; prefers: string[]; avoids: string[]; formal_lo: number; formal_hi: number; accents: number; patterns: boolean; staples: { kind: string; family: string; title: string; why: string }[] }
export interface Family { name: string; hex: string; neutral: boolean; earth?: boolean }
export interface Catalog { styles: StyleDef[]; occasions: Occasion[]; kinds: Kind[]; categories: { id: Category; label: string; plural: string }[]; families: Family[]; patterns: string[]; seasons: string[] }

export interface SuggestOut { suggestions: Score[]; items: Record<string, Item>; weather: DayWeather | null; weather_why?: string; occasion: Occasion; style: string; feel: number; empty?: string }

export interface DayEvent { title: string; time: string; end_time?: string; all_day?: boolean; place?: string; occasion: string; ref: string; url?: string }
export interface DayPlan {
  date: string; weather: DayWeather | null; weather_why?: string; events: DayEvent[]; events_why?: string
  training: { name: string; time?: string; done: boolean; url?: string } | null; outing: { title: string; activity: string; url?: string } | null
  occasion: string; because?: string; plan: Wear | null; worn: Wear | null; suggest: SuggestOut; tips: string[]; items: Record<string, Item>
}

export interface Gap { title: string; category: Category; kind: string; color_name: string; color_hex: string; why: string; style: string; unlocks: number; examples: Score[] }
export interface Idea { id: number; title: string; category: string; kind: string; color_hex: string; color_name: string; why: string; style: string; unlocks: number; price_hint: string; url: string; photo_id: number | null; status: 'idea' | 'wish' | 'bought' | 'dismissed'; made_by: string; wish_ref: string; wish_url: string; created_at: string; photo_url: string }
export interface ShopLink { name: string; url: string }

export interface PackItem { item_id: number; qty: number; packed: boolean }
export interface Pack { id: number; name: string; destination: string; lat: number | null; lng: number | null; start_date: string; end_date: string; purpose: string; items: PackItem[]; extras: { text: string; packed: boolean }[]; weather: DayWeather | null; notes: string; trip_ref: string; trip_url: string; created_at: string }
export interface Trip { ref: string; name: string; destination: string; start_date: string; end_date: string; lat: number | null; lng: number | null; url?: string }

export interface Purchase { app: 'money' | 'parcels'; ref: string; date: string; title: string; shop: string; amount: number | null; currency: string; status?: string; expected_on?: string; return_by?: string; url?: string; added: number }

export interface Summary { items: number; outfits: number; laundry: number; repair: number; lent: number; planned_tomorrow: boolean; worn_today: boolean; next_trip: { id: number; name: string; start_date: string; days: number; packed: number; total: number } | null; ideas: number; today: string }

/** What the AI says a photo shows. */
export interface AIPiece { name: string; category: Category; kind: string; colors: Color[]; pattern: string; material: string; fit: string; seasons: string[]; warmth: number; formality: number; styles: string[]; rain: boolean; brand: string; care: string; notes: string }
