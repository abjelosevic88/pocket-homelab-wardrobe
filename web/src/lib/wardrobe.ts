// Words, colours and the style catalogue for the wardrobe.
import { useEffect, useState } from 'react'
import { api, V1 } from './api'
import type { AppSlug, AppStateName, Catalog, Category, Item, ItemStatus } from './types'

export const APP_INFO: Record<AppSlug, { name: string; icon: string }> = {
  weather: { name: 'Pocket Weather', icon: 'cloud' },
  calendar: { name: 'Pocket Calendar', icon: 'calendar' },
  travel: { name: 'Pocket Travel', icon: 'plane' },
  trails: { name: 'Pocket Trails', icon: 'mountain' },
  fitness: { name: 'Pocket Fitness', icon: 'dumbbell' },
  wishlist: { name: 'Pocket Wishlist', icon: 'gift' },
  money: { name: 'Pocket Money', icon: 'wallet' },
  parcels: { name: 'Pocket Parcels', icon: 'package' },
  tasks: { name: 'Pocket Tasks', icon: 'check-square' },
  chat: { name: 'Pocket Chat', icon: 'message' },
  notes: { name: 'Pocket Notes', icon: 'note' },
}

export const STATE_WORDS: Record<AppStateName, string> = {
  on: 'connected', partly: 'partly connected', off: 'switched off', link_off: 'switched off in Pocket Core', missing: 'not on this box',
  old: 'needs an update', no_core: 'no Pocket Core', not_set_up: 'not set up', error: 'not reachable',
}

export const CATEGORY_ICON: Record<Category, string> = {
  top: 'shirt', knit: 'layers', bottom: 'ruler', dress: 'dress', outerwear: 'coat', shoes: 'shoe', bag: 'backpack', accessory: 'star', active: 'run', other: 'package',
}

export const STATUS: Record<ItemStatus, { label: string; icon: string; cls: string }> = {
  ready: { label: 'In the closet', icon: 'hanger', cls: 'st-ok' },
  laundry: { label: 'In the laundry', icon: 'washer', cls: 'st-due' },
  repair: { label: 'At the tailor / to repair', icon: 'scissors', cls: 'st-overdue' },
  lent: { label: 'Lent out', icon: 'users', cls: 'st-paused' },
  stored: { label: 'Stored away', icon: 'archive', cls: 'st-done' },
}

export const SEASON_WORDS: Record<string, string> = { spring: 'Spring', summer: 'Summer', autumn: 'Autumn', winter: 'Winter' }
export const PATTERN_WORDS: Record<string, string> = { plain: 'Plain', stripes: 'Stripes', checks: 'Checks', dots: 'Dots', floral: 'Floral', print: 'Print', other: 'Other' }
export const WARMTH_WORDS = ['None', 'Very light', 'Light', 'Medium', 'Warm', 'Very warm']
export const FORMAL_WORDS = ['', 'Sporty', 'Casual', 'Smart casual', 'Business', 'Black tie']
export const GONE_WORDS: Record<string, string> = { donated: 'Given away', sold: 'Sold', thrown: 'Thrown out', lost: 'Lost' }
export const PURPOSES: { value: string; label: string; icon: string }[] = [
  { value: 'holiday', label: 'Holiday', icon: 'sun' }, { value: 'city', label: 'City break', icon: 'building' }, { value: 'business', label: 'Work trip', icon: 'briefcase' },
  { value: 'beach', label: 'Beach', icon: 'waves' }, { value: 'ski', label: 'Ski', icon: 'snowflake' }, { value: 'outdoor', label: 'Outdoors', icon: 'mountain' },
  { value: 'wedding', label: 'Wedding', icon: 'gift' }, { value: 'other', label: 'Other', icon: 'suitcase' },
]

let catalog: Catalog | null = null
let loading: Promise<Catalog> | null = null

/** The styles, occasions, kinds and colours (loaded once). */
export function useCatalog(): Catalog | null {
  const [c, setC] = useState<Catalog | null>(catalog)
  useEffect(() => {
    if (catalog) return
    loading = loading || api.get<Catalog>(`${V1}/styles`).then(r => { catalog = r; return r })
    loading.then(setC).catch(() => { loading = null })
  }, [])
  return c
}

export function kindLabel(cat: Catalog | null, kind: string): string {
  return cat?.kinds.find(k => k.id === kind)?.label || kind
}
export function shapeOf(cat: Catalog | null, kind: string): string {
  return cat?.kinds.find(k => k.id === kind)?.shape || 'tshirt'
}
export function styleName(cat: Catalog | null, id: string): string {
  return cat?.styles.find(s => s.id === id)?.name || id
}
export function occasionLabel(cat: Catalog | null, id: string): string {
  return cat?.occasions.find(o => o.id === id)?.label || id
}
export function familyHex(cat: Catalog | null, name: string): string {
  return cat?.families.find(f => f.name === name)?.hex || '#888888'
}
export function catLabel(cat: Catalog | null, id: string, plural = false): string {
  const c = cat?.categories.find(x => x.id === id)
  return c ? (plural ? c.plural : c.label) : id
}

/** Black or white text on a colour. */
export function inkOn(hex: string): string {
  const h = hex.replace('#', '')
  if (h.length !== 6) return '#000'
  const r = parseInt(h.slice(0, 2), 16), g = parseInt(h.slice(2, 4), 16), b = parseInt(h.slice(4, 6), 16)
  return (r * 299 + g * 587 + b * 114) / 1000 > 150 ? '#1b1b1d' : '#ffffff'
}

/** Money in minor units as words ("€59.90"). */
export function money(minor: number | null | undefined, currency = 'EUR'): string {
  if (minor == null) return ''
  try { return new Intl.NumberFormat(undefined, { style: 'currency', currency: currency || 'EUR', maximumFractionDigits: minor % 100 === 0 ? 0 : 2 }).format(minor / 100) } catch { return (minor / 100).toFixed(2) + ' ' + currency }
}

/** The main colour of a piece. */
export function mainHex(it: Pick<Item, 'colors'>): string { return it.colors[0]?.hex || '#9a9ca0' }

/** "Worn 12 times, last 3 days ago". */
export function wornWords(it: Item): string {
  if (!it.wears) return 'Not worn yet'
  const t = it.wears === 1 ? 'Worn once' : `Worn ${it.wears} times`
  if (!it.last_worn) return t
  const days = Math.round((Date.now() - new Date(it.last_worn + 'T12:00:00').getTime()) / 86400000)
  return `${t}, last ${days <= 0 ? 'today' : days === 1 ? 'yesterday' : days < 31 ? `${days} days ago` : days < 365 ? `${Math.round(days / 30)} months ago` : 'over a year ago'}`
}

export function tempWords(w: { t_min?: number | null; t_max?: number | null } | null | undefined): string {
  if (!w) return ''
  if (w.t_min != null && w.t_max != null) return `${Math.round(w.t_min)}–${Math.round(w.t_max)}°C`
  if (w.t_max != null) return `${Math.round(w.t_max)}°C`
  return ''
}

export function weatherIcon(code: number | undefined): string {
  if (code == null) return 'cloud'
  if (code === 0) return 'sun'
  if (code <= 2) return 'sun'
  if (code >= 71 && code <= 77) return 'snowflake'
  if (code === 85 || code === 86) return 'snowflake'
  if ((code >= 51 && code <= 67) || (code >= 80 && code <= 82) || code >= 95) return 'cloud-rain'
  return 'cloud'
}
