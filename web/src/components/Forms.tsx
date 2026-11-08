// The dialogs: a piece (with its photo and the AI's reading of it),
// several pieces from one photo, what you wore, sharing to Pocket Chat,
// a packing list, picking pieces.
import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useAppState } from '../lib/data'
import { todayISO } from '../lib/format'
import { colorAt, shrink, uploadPhoto, type Shrunk } from '../lib/photo'
import type { AIPiece, Category, Color, Item, Pack, Trip } from '../lib/types'
import { FORMAL_WORDS, PATTERN_WORDS, PURPOSES, SEASON_WORDS, WARMTH_WORDS, catLabel, kindLabel, useCatalog } from '../lib/wardrobe'
import { Garment } from './Garment'
import { Icon } from './icons'
import { ItemThumb, OutfitBoard, Swatch } from './Pieces'
import { Select } from './Select'
import { Field, Modal, Segmented, Spinner, Switch, errMsg, useToast } from './ui'

export { Segmented }

export function Stars({ value, onChange, label = 'Rating' }: { value: number; onChange: (n: number) => void; label?: string }) {
  return (
    <span className="stars" role="radiogroup" aria-label={label}>
      {[1, 2, 3, 4, 5].map(n => <button key={n} type="button" role="radio" aria-checked={value === n} aria-label={`${n} of 5`} className={n <= value ? 'on' : ''} onClick={() => onChange(value === n ? 0 : n)}><Icon name="star" size={20} /></button>)}
    </span>
  )
}

/** Take or choose a picture; it is shrunk here, then sent. */
export function PhotoButton({ onPicked, label = 'Take or choose a photo', className = 'btn', kind = 'item', capture }: { onPicked: (s: Shrunk, photo: { id: number; url: string; thumb_url: string }) => void; label?: React.ReactNode; className?: string; kind?: 'item' | 'look' | 'check'; capture?: 'user' | 'environment' }) {
  const ref = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState(false)
  const { toast } = useToast()
  const pick = async (f: File | undefined) => {
    if (!f) return
    setBusy(true)
    try {
      const s = await shrink(f)
      const p = await uploadPhoto(s, kind)
      onPicked(s, p)
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false); if (ref.current) ref.current.value = '' }
  }
  return (
    <>
      <input ref={ref} type="file" accept="image/*" capture={capture} hidden onChange={e => pick(e.target.files?.[0])} />
      <button type="button" className={className} onClick={() => ref.current?.click()} disabled={busy}>{busy ? <Spinner /> : <Icon name="camera" size={15} />}{label}</button>
    </>
  )
}

type Draft = Omit<Partial<Item>, 'price'> & { price_text?: string }

const emptyDraft = (currency: string): Draft => ({ name: '', category: 'top', kind: 'tshirt', colors: [], pattern: 'plain', material: '', fit: '', seasons: [], warmth: 1, formality: 1, styles: [], rain: false, brand: '', size: '', price_text: '', currency, bought_on: '', shop: '', care: '', notes: '', favourite: false })

function fromAI(d: Draft, a: AIPiece): Draft {
  return { ...d, name: a.name || d.name, category: a.category, kind: a.kind, colors: a.colors.length ? a.colors : d.colors, pattern: a.pattern, material: a.material || d.material, fit: a.fit || d.fit,
    seasons: a.seasons, warmth: a.warmth, formality: a.formality, styles: a.styles, rain: a.rain, brand: a.brand || d.brand, care: a.care || d.care, notes: d.notes || a.notes }
}

export type ItemInitial = Draft & { photo_id?: number | null; photo_url?: string; source?: string; source_ref?: string }

/** A piece: new (with a photo the AI can read) or changed. */
export function ItemForm({ item, initial, onClose, onSaved }: { item?: Item; initial?: ItemInitial; onClose: () => void; onSaved?: (it: Item) => void }) {
  const { settings } = useApp()
  const cat = useCatalog()
  const { toast } = useToast()
  const navigate = useNavigate()
  const [d, setD] = useState<Draft>(() => item ? { ...item, price_text: item.price != null ? (item.price / 100).toFixed(2) : '' } : { ...emptyDraft(settings?.currency || 'EUR'), ...(initial || {}) })
  const [photo, setPhoto] = useState<{ id: number; url: string } | null>(item?.photo_id ? { id: item.photo_id, url: item.photo_url } : initial?.photo_id ? { id: initial.photo_id, url: initial.photo_url || `/api/v1/photos/${initial.photo_id}` } : null)
  const [palette, setPalette] = useState<string[]>([])
  const [aiBusy, setAiBusy] = useState(false)
  const [busy, setBusy] = useState(false)
  const [more, setMore] = useState(!!item)
  const set = (p: Partial<Draft>) => setD(x => ({ ...x, ...p }))
  const aiOn = !!settings?.ai_help && !!settings?.ai_photos

  const readPhoto = async (id: number) => {
    setAiBusy(true)
    try {
      const r = await api.post<{ item: AIPiece }>(`${V1}/ai/tag`, { photo_id: id })
      setD(x => fromAI(x, r.item))
      setMore(true)
      toast('Filled in from the photo — check it')
    } catch (e) { toast(errMsg(e), 'err') } finally { setAiBusy(false) }
  }
  const picked = (s: Shrunk, p: { id: number; url: string }) => {
    setPhoto({ id: p.id, url: p.url })
    setPalette(s.colors)
    if (!d.colors?.length && s.colors.length) set({ colors: s.colors.slice(0, 2).map(hex => ({ hex, name: '' })) })
    if (aiOn && !item) readPhoto(p.id)
  }
  const pickColor = (e: React.MouseEvent<HTMLImageElement>) => {
    const r = e.currentTarget.getBoundingClientRect()
    try {
      const hex = colorAt(e.currentTarget, e.clientX - r.left, e.clientY - r.top)
      set({ colors: [{ hex, name: '' }, ...(d.colors || []).slice(0, 2)] })
    } catch { /* a picture from elsewhere cannot be read */ }
  }
  const kinds = (cat?.kinds || []).filter(k => k.category === d.category)
  const save = async () => {
    setBusy(true)
    const price = d.price_text?.trim() ? Math.round(parseFloat(d.price_text.replace(',', '.')) * 100) : null
    const { price_text: _pt, ...rest } = d
    void _pt
    const body = { ...rest, price: price ?? undefined, no_price: price == null, photo_id: photo?.id ?? 0, colors: (d.colors || []).map(c => ({ hex: c.hex, name: c.name })) }
    try {
      const it = item ? await api.put<Item>(`${V1}/items/${item.id}`, body) : await api.post<Item>(`${V1}/items`, { ...body, source: initial?.source, source_ref: initial?.source_ref })
      announceChange()
      toast(item ? 'Saved' : 'Added to your wardrobe')
      onSaved?.(it)
      onClose()
      if (!item && !onSaved) navigate(`/closet/${it.id}`)
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  const toggle = (list: string[] = [], v: string) => list.includes(v) ? list.filter(x => x !== v) : [...list, v]
  return (
    <Modal title={item ? 'Change this piece' : 'Add a piece'} onClose={onClose} size="lg" footer={<>
      <button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" onClick={save} disabled={busy || aiBusy}>{item ? 'Save' : 'Add it'}</button>
    </>}>
      <div className="item-form">
        <div className="if-photo">
          {photo?.url ? <figure className="if-pic">
            <img src={photo.url} alt="" onClick={pickColor} />
            <figcaption className="muted tiny">Tap the picture to take a colour from it.</figcaption>
          </figure> : <div className="if-drawing"><Garment shape={cat?.kinds.find(k => k.id === d.kind)?.shape || 'tshirt'} color={d.colors?.[0]?.hex || '#9a9ca0'} pattern={d.pattern} size={150} /></div>}
          <div className="row wrap center">
            <PhotoButton onPicked={picked} label={photo ? 'Another photo' : 'Add a photo'} capture="environment" />
            {photo && aiOn && <button type="button" className="btn" onClick={() => readPhoto(photo.id)} disabled={aiBusy}>{aiBusy ? <Spinner /> : <Icon name="sparkles" size={15} />}Read it with AI</button>}
            {photo && <button type="button" className="btn ghost" onClick={() => setPhoto(null)}>Remove photo</button>}
          </div>
          {aiBusy && <p className="muted small center"><Spinner /> The AI is looking at the photo…</p>}
          {!aiOn && !item && <p className="muted tiny center">With AI and photos switched on (Settings → AI) a photo fills in the rest.</p>}
        </div>
        <div className="if-fields stack">
          <Field label="What is it?">
            <div className="two-col tight">
              <Select ariaLabel="Category" value={d.category || 'top'} onChange={v => { const first = (cat?.kinds || []).find(k => k.category === v); set({ category: v as Category, kind: first?.id || 'other', warmth: first?.warmth ?? d.warmth, formality: first?.formality ?? d.formality }) }} options={(cat?.categories || []).map(c => ({ value: c.id, label: c.label }))} />
              <Select ariaLabel="Kind" value={d.kind || ''} onChange={v => { const k = cat?.kinds.find(x => x.id === v); set({ kind: v, warmth: k?.warmth ?? d.warmth, formality: k?.formality ?? d.formality, rain: k?.rain ?? d.rain }) }} options={kinds.map(k => ({ value: k.id, label: k.label }))} />
            </div>
          </Field>
          <Field label="Name" help="Leave it empty for “Navy jumper” and the like."><input value={d.name || ''} onChange={e => set({ name: e.target.value })} placeholder={`${d.colors?.[0]?.name || ''} ${kindLabel(cat, d.kind || '').toLowerCase()}`.trim()} /></Field>
          <Field label="Colours" help="The main one first. Tap a swatch to remove it.">
            <div className="row wrap tight">
              {(d.colors || []).map((c, i) => <Swatch key={i} hex={c.hex} name={c.name || c.hex} on onClick={() => set({ colors: (d.colors || []).filter((_, j) => j !== i) })} />)}
              {palette.filter(h => !(d.colors || []).some(c => c.hex === h)).map(h => <button key={h} type="button" className="swatch-btn add" style={{ background: h }} title="From the photo" onClick={() => set({ colors: [...(d.colors || []), { hex: h, name: '' }].slice(0, 3) })}><Icon name="plus" size={12} /></button>)}
              <ColorMenu onPick={(c: Color) => set({ colors: [...(d.colors || []), c].slice(0, 3) })} />
            </div>
          </Field>
          <Field label="Pattern"><Select ariaLabel="Pattern" value={d.pattern || 'plain'} onChange={v => set({ pattern: v })} options={Object.entries(PATTERN_WORDS).map(([value, label]) => ({ value, label }))} /></Field>
          <div className="two-col tight">
            <Field label="How warm" help={WARMTH_WORDS[d.warmth ?? 1]}><input type="range" min={0} max={5} value={d.warmth ?? 1} onChange={e => set({ warmth: Number(e.target.value) })} aria-label="How warm" /></Field>
            <Field label="How smart" help={FORMAL_WORDS[d.formality ?? 1]}><input type="range" min={1} max={5} value={d.formality ?? 1} onChange={e => set({ formality: Number(e.target.value) })} aria-label="How smart" /></Field>
          </div>
          {!more && <button type="button" className="link-btn" onClick={() => setMore(true)}>More: seasons, style, size, price, care…</button>}
          {more && <>
            <Field label="Seasons" help="None picked: all year.">
              <div className="chips">{(cat?.seasons || []).map(s => <button key={s} type="button" className={`chip ${d.seasons?.includes(s) ? 'on' : ''}`} onClick={() => set({ seasons: toggle(d.seasons, s) })}>{SEASON_WORDS[s]}</button>)}</div>
            </Field>
            <Field label="Styles it fits">
              <div className="chips">{(cat?.styles || []).map(s => <button key={s.id} type="button" className={`chip ${d.styles?.includes(s.id) ? 'on' : ''}`} onClick={() => set({ styles: toggle(d.styles, s.id) })}>{s.name}</button>)}</div>
            </Field>
            <div className="two-col tight">
              <Field label="Brand"><input value={d.brand || ''} onChange={e => set({ brand: e.target.value })} /></Field>
              <Field label="Size"><input value={d.size || ''} onChange={e => set({ size: e.target.value })} placeholder={d.category === 'shoes' ? settings?.size_shoe : d.category === 'bottom' ? settings?.size_bottom : settings?.size_top} /></Field>
            </div>
            <div className="two-col tight">
              <Field label="Material"><input value={d.material || ''} onChange={e => set({ material: e.target.value })} placeholder="cotton, wool…" /></Field>
              <Field label="Fit"><Select ariaLabel="Fit" value={d.fit || ''} onChange={v => set({ fit: v })} options={[{ value: '', label: '—' }, { value: 'slim', label: 'Slim' }, { value: 'regular', label: 'Regular' }, { value: 'relaxed', label: 'Relaxed' }, { value: 'oversized', label: 'Oversized' }]} /></Field>
            </div>
            <div className="three-col tight">
              <Field label="Price"><div className="input-group"><input inputMode="decimal" value={d.price_text || ''} onChange={e => set({ price_text: e.target.value })} placeholder="0.00" /><span className="input-suffix">{d.currency}</span></div></Field>
              <Field label="Bought on"><input type="date" value={d.bought_on || ''} onChange={e => set({ bought_on: e.target.value })} max={todayISO()} /></Field>
              <Field label="Shop"><input value={d.shop || ''} onChange={e => set({ shop: e.target.value })} /></Field>
            </div>
            <Field label="Care"><input value={d.care || ''} onChange={e => set({ care: e.target.value })} placeholder="Wash at 30°, dry flat" /></Field>
            <Field label="Notes"><textarea rows={2} value={d.notes || ''} onChange={e => set({ notes: e.target.value })} /></Field>
            <div className="row wrap">
              <label className="row tight"><Switch checked={!!d.rain} onChange={v => set({ rain: v })} label="Keeps rain out" /><span>Keeps rain out</span></label>
              <label className="row tight"><Switch checked={!!d.favourite} onChange={v => set({ favourite: v })} label="Favourite" /><span>Favourite</span></label>
            </div>
          </>}
        </div>
      </div>
    </Modal>
  )
}

/** The colour families as a menu of swatches. */
function ColorMenu({ onPick }: { onPick: (c: Color) => void }) {
  const cat = useCatalog()
  const [open, setOpen] = useState(false)
  return (
    <span className="color-menu">
      <button type="button" className="btn sm" onClick={() => setOpen(o => !o)} aria-expanded={open}><Icon name="palette" size={14} />Pick</button>
      {open && <span className="color-menu-list">{(cat?.families || []).map(f => <Swatch key={f.name} hex={f.hex} name={f.name} size={26} onClick={() => { onPick({ hex: f.hex, name: f.name }); setOpen(false) }} />)}
        <label className="swatch-btn custom" title="Any colour"><input type="color" onChange={e => { onPick({ hex: e.target.value, name: '' }); setOpen(false) }} /><Icon name="plus" size={12} /></label>
      </span>}
    </span>
  )
}

/** Several pieces from one photo (a rail, a bed full of clothes). */
export function ManyFromPhoto({ onClose }: { onClose: () => void }) {
  const { settings } = useApp()
  const { toast } = useToast()
  const cat = useCatalog()
  const [photo, setPhoto] = useState<{ id: number; url: string } | null>(null)
  const [found, setFound] = useState<(AIPiece & { keep: boolean })[] | null>(null)
  const [busy, setBusy] = useState(false)
  const read = async (id: number) => {
    setBusy(true)
    try { const r = await api.post<{ items: AIPiece[] }>(`${V1}/ai/tag`, { photo_id: id, many: true }); setFound(r.items.map(x => ({ ...x, keep: true }))) } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  const save = async () => {
    if (!found) return
    setBusy(true)
    let n = 0
    for (const p of found.filter(x => x.keep)) {
      try { await api.post(`${V1}/items`, { ...p, source: 'ai', currency: settings?.currency }); n++ } catch { /* the rest still go in */ }
    }
    setBusy(false)
    announceChange()
    toast(`${n} pieces added`)
    onClose()
  }
  const aiOn = !!settings?.ai_help && !!settings?.ai_photos
  return (
    <Modal title="Several pieces from one photo" onClose={onClose} size="lg" footer={<>
      <button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" onClick={save} disabled={busy || !found?.some(x => x.keep)}>Add {found?.filter(x => x.keep).length || ''} pieces</button>
    </>}>
      {!aiOn && <div className="callout info"><Icon name="info" size={15} /><div>This needs the AI with photos switched on (Settings → AI).</div></div>}
      <p className="muted">Lay your clothes out on a bed or photograph a rail. The AI lists every piece it sees; untick what you do not want. Each gets its own drawing — add a photo of it later if you like.</p>
      <div className="row wrap">
        <PhotoButton onPicked={(_, p) => { setPhoto(p); if (aiOn) read(p.id) }} label={photo ? 'Another photo' : 'Take or choose the photo'} />
      </div>
      {photo && <img className="many-pic" src={photo.url} alt="" />}
      {busy && !found && <p className="muted"><Spinner /> Looking at the photo…</p>}
      {found && <div className="many-list">
        {found.map((p, i) => (
          <label key={i} className={`many-row ${p.keep ? '' : 'off'}`}>
            <input type="checkbox" checked={p.keep} onChange={e => setFound(found.map((x, j) => j === i ? { ...x, keep: e.target.checked } : x))} />
            <Garment shape={cat?.kinds.find(k => k.id === p.kind)?.shape || 'tshirt'} color={p.colors[0]?.hex || '#999'} pattern={p.pattern} size={48} />
            <span className="grow"><b>{p.name}</b><span className="muted small"> · {catLabel(cat, p.category)}{p.material ? ` · ${p.material}` : ''}</span></span>
          </label>
        ))}
        {!found.length && <p className="muted">Nothing found in that photo.</p>}
      </div>}
    </Modal>
  )
}

/** "I wore this" (or plan it for a day). */
export function WearForm({ ids, items, outfitId, date: d0, planned: p0 = false, occasion = '', event = '', onClose, onSaved }: { ids: number[]; items: Record<string, Item>; outfitId?: number; date?: string; planned?: boolean; occasion?: string; event?: string; onClose: () => void; onSaved?: () => void }) {
  const { toast } = useToast()
  const [date, setDate] = useState(d0 || todayISO())
  const [feel, setFeel] = useState(0)
  const [notes, setNotes] = useState('')
  const [photo, setPhoto] = useState<{ id: number; url: string } | null>(null)
  const [busy, setBusy] = useState(false)
  const planned = date > todayISO() || (p0 && date >= todayISO())
  const save = async () => {
    setBusy(true)
    try {
      await api.post(`${V1}/wears`, { date, planned, item_ids: ids, outfit_id: outfitId || 0, occasion, event, feel, notes, photo_id: photo?.id || 0 })
      announceChange()
      toast(planned ? 'Planned' : 'Logged')
      onSaved?.()
      onClose()
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title={planned ? 'Plan this outfit' : 'I wore this'} onClose={onClose} footer={<>
      <button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" onClick={save} disabled={busy}>{planned ? 'Plan it' : 'Log it'}</button>
    </>}>
      <OutfitBoard ids={ids} items={items} size="sm" />
      <div className="stack mt">
        <Field label="Day"><input type="date" value={date} onChange={e => setDate(e.target.value)} /></Field>
        {!planned && <>
          <Field label="How did it feel?"><Stars value={feel} onChange={setFeel} label="How it felt" /></Field>
          <Field label="A photo of the look (optional)">
            <div className="row wrap">{photo && <img src={photo.url} alt="" className="look-mini" />}<PhotoButton kind="look" capture="user" onPicked={(_, p) => setPhoto(p)} label={photo ? 'Another' : 'Mirror photo'} /></div>
          </Field>
          <Field label="Notes"><input value={notes} onChange={e => setNotes(e.target.value)} placeholder="Got compliments; too warm by noon…" /></Field>
        </>}
      </div>
    </Modal>
  )
}

/** Ask someone in Pocket Chat what they think. */
export function ChatShare({ ids, outfitId, title, onClose }: { ids: number[]; outfitId?: number; title?: string; onClose: () => void }) {
  const { toast } = useToast()
  const appState = useAppState()
  const chat = appState('chat', 'chat:write')
  const [chats, setChats] = useState<{ id: string; name: string; emoji: string }[] | null>(null)
  const [to, setTo] = useState('me')
  const [text, setText] = useState('What do you think of this?')
  const [busy, setBusy] = useState(false)
  useEffect(() => { if (chat.ok) api.get<{ conversations: { id: string; name: string; emoji: string }[] }>(`${V1}/apps/chats`).then(r => setChats(r.conversations)).catch(() => setChats([])) }, [chat.ok])
  const send = async () => {
    setBusy(true)
    try { const r = await api.post<{ text: string }>(`${V1}/apps/chat`, { to, text, item_ids: ids, outfit_id: outfitId || 0, title }); toast(r.text); onClose() } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title="Ask in Pocket Chat" onClose={onClose} size="sm" footer={<>
      <button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" onClick={send} disabled={busy || !chat.ok}>Send</button>
    </>}>
      {!chat.ok ? <p className="muted">{chat.app?.detail || 'Pocket Chat is not connected'}. {chat.app?.fix}</p> : <div className="stack">
        <Field label="To"><Select ariaLabel="Conversation" value={to} onChange={setTo} options={[{ value: 'me', label: 'Just me (saved for later)' }, ...(chats || []).map(c => ({ value: c.id, label: `${c.emoji || ''} ${c.name}`.trim() }))]} /></Field>
        <Field label="Message"><input value={text} onChange={e => setText(e.target.value)} /></Field>
        <p className="muted small">They see the outfit's picture and its pieces; the picture opens without signing in, for whoever has the link.</p>
      </div>}
    </Modal>
  )
}

/** A packing list: from a Pocket Travel trip, or typed in. */
export function PackForm({ pack, onClose, onSaved }: { pack?: Pack; onClose: () => void; onSaved?: (p: Pack) => void }) {
  const { toast } = useToast()
  const navigate = useNavigate()
  const appState = useAppState()
  const travel = appState('travel', 'trips:read')
  const [trips, setTrips] = useState<Trip[] | null>(null)
  const [f, setF] = useState({ name: pack?.name || '', destination: pack?.destination || '', start_date: pack?.start_date || '', end_date: pack?.end_date || '', purpose: pack?.purpose || 'holiday', notes: pack?.notes || '', lat: (pack?.lat ?? null) as number | null, lng: (pack?.lng ?? null) as number | null, trip_ref: pack?.trip_ref || '', trip_url: pack?.trip_url || '' })
  const [busy, setBusy] = useState(false)
  useEffect(() => { if (travel.ok && !pack) api.get<{ trips: Trip[] }>(`${V1}/trips`).then(r => setTrips(r.trips)).catch(() => setTrips([])) }, [travel.ok, pack])
  const fromTrip = (t: Trip) => setF({ ...f, name: t.name, destination: t.destination, start_date: t.start_date, end_date: t.end_date, lat: t.lat, lng: t.lng, trip_ref: t.ref, trip_url: t.url || '' })
  const save = async () => {
    setBusy(true)
    try {
      const p = pack ? await api.put<Pack>(`${V1}/packs/${pack.id}`, f) : await api.post<Pack>(`${V1}/packs`, f)
      announceChange(); onSaved?.(p); onClose()
      if (!pack && !onSaved) navigate(`/packing/${p.id}`)
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title={pack ? 'The trip' : 'Pack for a trip'} onClose={onClose} footer={<>
      <button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" onClick={save} disabled={busy || (!f.name.trim() && !f.destination.trim())}>{pack ? 'Save' : 'Start packing'}</button>
    </>}>
      <div className="stack">
        {!pack && (travel.ok ? (trips?.length ? <Field label="From Pocket Travel"><div className="chips">{trips.map(t => <button key={t.ref} type="button" className={`chip ${f.trip_ref === t.ref ? 'on' : ''}`} onClick={() => fromTrip(t)}><Icon name="plane" size={13} />{t.name}</button>)}</div></Field> : trips ? <p className="muted small">No coming trips in Pocket Travel.</p> : null)
          : <p className="muted small"><Icon name="plane" size={13} className="inline-ico" /> With Pocket Travel connected, a trip fills this in (and its weather).</p>)}
        <div className="two-col tight">
          <Field label="Name"><input value={f.name} onChange={e => setF({ ...f, name: e.target.value })} placeholder="Barcelona" /></Field>
          <Field label="Where"><input value={f.destination} onChange={e => setF({ ...f, destination: e.target.value })} placeholder="Barcelona, Spain" /></Field>
        </div>
        <div className="two-col tight">
          <Field label="From"><input type="date" value={f.start_date} onChange={e => setF({ ...f, start_date: e.target.value })} /></Field>
          <Field label="To"><input type="date" value={f.end_date} onChange={e => setF({ ...f, end_date: e.target.value })} min={f.start_date} /></Field>
        </div>
        <Field label="Kind of trip"><div className="chips">{PURPOSES.map(p => <button key={p.value} type="button" className={`chip ${f.purpose === p.value ? 'on' : ''}`} onClick={() => setF({ ...f, purpose: p.value })}><Icon name={p.icon} size={13} />{p.label}</button>)}</div></Field>
        <Field label="Plans" help="A wedding on Saturday, a hike, dinners… the AI packs for them."><textarea rows={2} value={f.notes} onChange={e => setF({ ...f, notes: e.target.value })} /></Field>
      </div>
    </Modal>
  )
}

/** Pick pieces from the closet (for an outfit, a packing list). */
export function PiecePicker({ items, chosen, onChange, categories }: { items: Item[]; chosen: number[]; onChange: (ids: number[]) => void; categories?: Category[] }) {
  const cat = useCatalog()
  const [tab, setTab] = useState<string>('all')
  const cats = (cat?.categories || []).filter(c => items.some(i => i.category === c.id) && (!categories || categories.includes(c.id)))
  const list = items.filter(i => tab === 'all' || i.category === tab)
  return (
    <div className="picker-wrap">
      <div className="chips scroll-x">
        <button type="button" className={`chip ${tab === 'all' ? 'on' : ''}`} onClick={() => setTab('all')}>All</button>
        {cats.map(c => <button key={c.id} type="button" className={`chip ${tab === c.id ? 'on' : ''}`} onClick={() => setTab(c.id)}>{c.plural}</button>)}
      </div>
      <div className="pick-grid">
        {list.map(it => {
          const on = chosen.includes(it.id)
          return (
            <button key={it.id} type="button" className={`pick-cell ${on ? 'on' : ''} ${it.status !== 'ready' ? 'away' : ''}`} onClick={() => onChange(on ? chosen.filter(x => x !== it.id) : [...chosen, it.id])} aria-pressed={on} title={it.status !== 'ready' ? `${it.name} — not in the closet` : it.name}>
              <ItemThumb item={it} size={72} />
              {on && <span className="pick-check"><Icon name="check" size={13} /></span>}
              <span className="pick-name ellipsis">{it.name}</span>
            </button>
          )
        })}
      </div>
    </div>
  )
}
