// How pieces and outfits look everywhere: a photo when there is one, else
// the drawing; outfits laid out flat like on a bed.
import { Link } from 'react-router-dom'
import type { DayWeather, Item, Score, ShopLink } from '../lib/types'
import { inkOn, mainHex, shapeOf, tempWords, useCatalog, weatherIcon } from '../lib/wardrobe'
import { Garment } from './Garment'
import { Icon } from './icons'
import { useIsMobile } from './ui'

/** One piece as a square: its photo, or its drawing in its colour. */
export function ItemThumb({ item, size = 96, className = '' }: { item: Pick<Item, 'kind' | 'colors' | 'pattern' | 'thumb_url' | 'name'>; size?: number; className?: string }) {
  const cat = useCatalog()
  if (item.thumb_url) return <img className={`item-thumb ${className}`} src={item.thumb_url} alt={item.name} width={size} height={size} loading="lazy" style={{ width: size, height: size }} />
  return <span className={`item-thumb drawn ${className}`} style={{ width: size, height: size }}><Garment shape={shapeOf(cat, item.kind)} color={mainHex(item)} second={item.colors[1]?.hex} pattern={item.pattern} size={Math.round(size * 0.86)} title={item.name} /></span>
}

/** A piece in a grid: picture, name, a line about it. */
export function ItemCard({ item, sub, to, onClick, selected, badge }: { item: Item; sub?: React.ReactNode; to?: string; onClick?: () => void; selected?: boolean; badge?: React.ReactNode }) {
  const body = <>
    <span className="ic-pic"><ItemThumb item={item} size={140} />{badge}{item.favourite && <span className="ic-fav" title="Favourite"><Icon name="heart" size={12} /></span>}</span>
    <span className="ic-name ellipsis">{item.name}</span>
    {sub && <span className="ic-sub ellipsis">{sub}</span>}
  </>
  if (to) return <Link to={to} className={`item-card ${selected ? 'on' : ''}`}>{body}</Link>
  return <button type="button" className={`item-card ${selected ? 'on' : ''}`} onClick={onClick} aria-pressed={selected}>{body}</button>
}

const ORDER: Record<string, number> = { outerwear: 0, knit: 1, top: 2, dress: 2, active: 2, bottom: 3, shoes: 4, bag: 5, accessory: 6, other: 7 }

/** An outfit laid out flat: layers on the left, the rest beside them. */
export function OutfitBoard({ ids, items, size = 'md', idea }: { ids: number[]; items: Record<string, Item> | Map<number, Item>; size?: 'sm' | 'md' | 'lg'; idea?: { kind: string; color_hex: string; title: string } }) {
  const cat = useCatalog()
  const get = (id: number) => items instanceof Map ? items.get(id) : items[String(id)]
  const list = ids.map(id => ({ id, it: get(id) })).filter(x => x.it || (x.id === -1 && idea)).sort((a, b) => ORDER[a.it?.category || (idea ? (cat?.kinds.find(k => k.id === idea.kind)?.category || 'other') : 'other')] - ORDER[b.it?.category || 'other'])
  const mobile = useIsMobile()
  const px = size === 'sm' ? 54 : size === 'lg' ? (mobile ? 100 : 120) : (mobile ? 66 : 84)
  return (
    <div className={`board board-${size} n${Math.min(list.length, 6)}`}>
      {list.map(({ id, it }) => id === -1 && idea
        ? <span key="idea" className="board-cell idea" title={`${idea.title} (to buy)`}><span className="item-thumb drawn" style={{ width: px, height: px }}><Garment shape={shapeOf(cat, idea.kind)} color={idea.color_hex} size={Math.round(px * 0.86)} title={idea.title} /></span><span className="board-new">new</span></span>
        : <span key={id} className="board-cell" title={it!.name}><ItemThumb item={it!} size={px} /></span>)}
    </div>
  )
}

/** A score out of 100 in words and a colour. */
export function ScoreBadge({ score }: { score: number }) {
  const cls = score >= 85 ? 'great' : score >= 70 ? 'good' : score >= 55 ? 'ok' : 'weak'
  const words = score >= 85 ? 'Great match' : score >= 70 ? 'Works well' : score >= 55 ? 'Fine' : 'Not quite'
  return <span className={`score-badge ${cls}`} title={`${Math.round(score)} out of 100`}><b>{Math.round(score)}</b>{words}</span>
}

/** Why an outfit works, and what does not. */
export function Reasons({ s, max = 3 }: { s: Pick<Score, 'reasons' | 'warnings'>; max?: number }) {
  return (
    <ul className="reasons">
      {s.reasons.slice(0, max).map(r => <li key={r} className="good"><Icon name="check" size={13} />{r}</li>)}
      {s.warnings.slice(0, max).map(r => <li key={r} className="warn"><Icon name="alert" size={13} />{r}</li>)}
    </ul>
  )
}

/** Colour swatches with their names. */
export function ColorDots({ colors, size = 14 }: { colors: { hex: string; name: string }[]; size?: number }) {
  return <span className="color-dots">{colors.map((c, i) => <span key={i} className="color-dot" style={{ width: size, height: size, background: c.hex }} title={c.name} />)}</span>
}

export function Swatch({ hex, name, on, onClick, size = 30 }: { hex: string; name: string; on?: boolean; onClick?: () => void; size?: number }) {
  return <button type="button" className={`swatch-btn ${on ? 'on' : ''}`} onClick={onClick} title={name} aria-pressed={on} style={{ width: size, height: size, background: hex, color: inkOn(hex) }}>{on && <Icon name="check" size={14} />}</button>
}

/** "Look for it" in the person's shops. */
export function ShopLinks({ shops }: { shops: ShopLink[] }) {
  if (!shops.length) return null
  return <div className="shop-links">{shops.map(s => <a key={s.url} className="btn sm" href={s.url} target="_blank" rel="noreferrer"><Icon name="search" size={13} />{s.name}</a>)}</div>
}

/** The day's weather in one line. */
export function WeatherChip({ w, why }: { w: DayWeather | null | undefined; why?: string }) {
  if (!w) return why ? <span className="weather-chip off" title={why}><Icon name="cloud" size={15} />No forecast</span> : null
  return <span className="weather-chip" title={w.text || w.condition}><Icon name={weatherIcon(w.code)} size={15} />{tempWords(w)}{w.condition && <span className="muted">· {w.condition.toLowerCase()}</span>}{w.place && <span className="muted">· {w.place}</span>}</span>
}
