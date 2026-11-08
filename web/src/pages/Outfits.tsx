// Saved outfits as flat-lays, the outfit maker (pick pieces, see at once
// how well they go), and ideas for any day, style and occasion.
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { announceChange, useAppState, useOnChange } from '../lib/data'
import { fmtDate, todayISO } from '../lib/format'
import type { Item, Outfit, Score, SuggestOut } from '../lib/types'
import { SEASON_WORDS, occasionLabel, styleName, useCatalog } from '../lib/wardrobe'
import { useCrumb } from '../components/Layout'
import { ChatShare, PiecePicker, Stars, WearForm } from '../components/Forms'
import { OutfitBoard, Reasons, ScoreBadge, WeatherChip } from '../components/Pieces'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { Card, Confirm, Empty, Field, Loading, Spinner, Switch, errMsg, useAsync, useDebounce, useToast } from '../components/ui'

interface ListOut { outfits: Outfit[]; items: Record<string, Item> }

export default function Outfits() {
  const cat = useCatalog()
  const [occ, setOcc] = useState('')
  const { data, reload } = useAsync(() => api.get<ListOut>(`${V1}/outfits`), [])
  useOnChange(useCallback(() => { reload() }, [reload]))
  if (!data) return <Loading />
  const list = data.outfits.filter(o => !occ || o.occasion === occ)
  const used = Array.from(new Set(data.outfits.map(o => o.occasion).filter(Boolean)))
  return (
    <div className="page">
      <div className="page-head row wrap">
        <div className="grow"><h2 className="page-title">Outfits</h2><p className="muted">{data.outfits.length} saved</p></div>
        <Link className="btn" to="/outfits/ideas"><Icon name="shuffle" size={15} />Ideas</Link>
        <Link className="btn primary" to="/outfits/new"><Icon name="plus" size={15} />Make an outfit</Link>
      </div>
      {used.length > 1 && <div className="chips scroll-x">
        <button type="button" className={`chip ${!occ ? 'on' : ''}`} onClick={() => setOcc('')}>All</button>
        {used.map(o => <button key={o} type="button" className={`chip ${occ === o ? 'on' : ''}`} onClick={() => setOcc(o)}>{occasionLabel(cat, o)}</button>)}
      </div>}
      {list.length === 0 ? <Empty title="No outfits yet" icon="layers"><p>Put pieces together yourself, or let the ideas page suggest combinations for a style, an occasion and the weather.</p><div className="row wrap center"><Link className="btn primary" to="/outfits/new">Make an outfit</Link><Link className="btn" to="/outfits/ideas">See ideas</Link></div></Empty>
        : <div className="outfit-grid">
          {list.map(o => <Link key={o.id} to={`/outfits/${o.id}`} className="outfit-card">
            <OutfitBoard ids={o.item_ids} items={data.items} size="md" />
            <span className="ic-name ellipsis">{o.favourite && <Icon name="heart" size={12} className="inline-ico accent-text" />} {o.name}</span>
            <span className="ic-sub ellipsis">{[o.occasion && occasionLabel(cat, o.occasion), o.wears ? `worn ${o.wears}×` : 'not worn yet'].filter(Boolean).join(' · ')}</span>
          </Link>)}
        </div>}
    </div>
  )
}

/** Put pieces together; the score and the reasons change as you pick. */
export function OutfitMaker() {
  const { id } = useParams()
  const navigate = useNavigate()
  const cat = useCatalog()
  const { toast } = useToast()
  const [params] = useSearchParams()
  const { data: all } = useAsync(() => api.get<{ items: Item[] }>(`${V1}/items`), [])
  const { data: existing } = useAsync(() => id ? api.get<{ outfit: Outfit }>(`${V1}/outfits/${id}`) : Promise.resolve(null), [id])
  const [chosen, setChosen] = useState<number[]>(() => (params.get('items') || '').split(',').map(Number).filter(Boolean))
  const [name, setName] = useState('')
  const [occasion, setOccasion] = useState('everyday')
  const [styleId, setStyleId] = useState('')
  const [forWeather, setForWeather] = useState(false)
  const [rating, setRating] = useState<{ rating: Score } | null>(null)
  const [busy, setBusy] = useState(false)
  useEffect(() => { if (existing?.outfit) { setChosen(existing.outfit.item_ids); setName(existing.outfit.name); setOccasion(existing.outfit.occasion || 'everyday'); setStyleId(existing.outfit.style) } }, [existing])
  const key = useDebounce(chosen.join(',') + occasion + styleId + forWeather, 200)
  useEffect(() => {
    if (!chosen.length) { setRating(null); return }
    api.post<{ rating: Score }>(`${V1}/rate`, { item_ids: chosen, occasion, style: styleId, weather: forWeather }).then(setRating).catch(() => {})
  }, [key]) // eslint-disable-line react-hooks/exhaustive-deps
  useCrumb(id ? 'Change' : 'New')
  const items = useMemo(() => Object.fromEntries((all?.items || []).map(i => [String(i.id), i])), [all])
  if (!all) return <Loading />
  const save = async () => {
    setBusy(true)
    try {
      const body = { item_ids: chosen, name, occasion, style: styleId }
      const o = id ? await api.put<Outfit>(`${V1}/outfits/${id}`, body) : await api.post<Outfit>(`${V1}/outfits`, body)
      announceChange(); toast('Saved'); navigate(`/outfits/${o.id}`)
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  return (
    <div className="page maker">
      <div className="maker-grid">
        <div className="maker-board stack">
          <Card title={id ? 'Change the outfit' : 'Make an outfit'} actions={<button type="button" className="btn primary" onClick={save} disabled={busy || !chosen.length}>Save</button>}>
            {chosen.length ? <OutfitBoard ids={chosen} items={items} size="lg" /> : <p className="muted">Pick pieces below — a top and a bottom (or a dress), shoes, and a layer if it is cold.</p>}
            {rating && chosen.length > 0 && <div className="stack mt"><ScoreBadge score={rating.rating.score} /><Reasons s={rating.rating} max={4} /></div>}
          </Card>
          <Card>
            <div className="stack">
              <Field label="Name"><input value={name} onChange={e => setName(e.target.value)} placeholder="Leave empty for one from its pieces" /></Field>
              <div className="two-col tight">
                <Field label="For"><Select ariaLabel="Occasion" value={occasion} onChange={setOccasion} options={(cat?.occasions || []).map(o => ({ value: o.id, label: o.label }))} /></Field>
                <Field label="Style"><Select ariaLabel="Style" value={styleId} onChange={setStyleId} options={[{ value: '', label: 'Your own' }, ...(cat?.styles || []).map(s => ({ value: s.id, label: s.name }))]} /></Field>
              </div>
              <label className="row tight"><Switch checked={forWeather} onChange={setForWeather} label="Check against today's weather" /><span>Check it against today's weather</span></label>
            </div>
          </Card>
        </div>
        <Card title="Your pieces" className="min0"><PiecePicker items={all.items} chosen={chosen} onChange={setChosen} /></Card>
      </div>
    </div>
  )
}

/** One saved outfit. */
export function OutfitPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const cat = useCatalog()
  const { toast } = useToast()
  const appState = useAppState()
  const chat = appState('chat', 'chat:write')
  const { data, reload } = useAsync(() => api.get<{ outfit: Outfit; items: Record<string, Item>; rating: Score; share: string }>(`${V1}/outfits/${id}`), [id])
  useOnChange(useCallback(() => { reload() }, [reload]))
  const [wear, setWear] = useState<boolean | null>(null)
  const [share, setShare] = useState(false)
  const [del, setDel] = useState(false)
  useCrumb(data?.outfit.name)
  if (!data) return <Loading />
  const o = data.outfit
  const put = async (patch: Partial<Outfit>) => { try { await api.put(`${V1}/outfits/${o.id}`, patch); announceChange() } catch (e) { toast(errMsg(e), 'err') } }
  return (
    <div className="page det-page">
      <div className="outfit-top">
        <OutfitBoard ids={o.item_ids} items={data.items} size="lg" />
        <div className="stack min0">
          <div className="det-kicker">{[o.occasion && occasionLabel(cat, o.occasion), o.style && styleName(cat, o.style), o.made_by === 'ai' ? 'by the AI' : o.made_by === 'ideas' ? 'from the ideas' : ''].filter(Boolean).join(' · ')}</div>
          <h2 className="det-title">{o.name}</h2>
          <ScoreBadge score={data.rating.score} />
          <Reasons s={data.rating} />
          <div className="row tight"><span className="muted small">Your rating</span><Stars value={o.rating} onChange={n => put({ rating: n })} /></div>
          <p className="muted small">{o.wears ? `Worn ${o.wears}×, last ${fmtDate(o.last_worn)}` : 'Not worn yet'}{o.seasons.length ? ` · ${o.seasons.map(s => SEASON_WORDS[s]).join(', ')}` : ''}</p>
          <div className="row wrap">
            <button type="button" className="btn primary" onClick={() => setWear(false)}><Icon name="check" size={15} />Wear it today</button>
            <button type="button" className="btn" onClick={() => setWear(true)}><Icon name="calendar-plus" size={15} />Plan it</button>
            <button type="button" className={`btn ${o.favourite ? 'on' : ''}`} onClick={() => put({ favourite: !o.favourite })} aria-pressed={o.favourite}><Icon name="heart" size={15} />{o.favourite ? 'Favourite' : 'Love it'}</button>
            <button type="button" className="btn" onClick={() => setShare(true)} disabled={!chat.ok} title={chat.ok ? 'Ask someone in Pocket Chat' : `Pocket Chat: ${chat.app?.detail || 'not connected'}`}><Icon name="message" size={15} />Ask in Chat</button>
            <Link className="btn" to={`/outfits/${o.id}/edit`}><Icon name="edit" size={15} />Change</Link>
            <button type="button" className="btn ghost danger-text" onClick={() => setDel(true)}><Icon name="trash" size={15} />Delete</button>
          </div>
          {o.notes && <p>{o.notes}</p>}
        </div>
      </div>
      <Card title="The pieces"><ul className="mini-list">{o.item_ids.map(i => data.items[String(i)]).filter(Boolean).map(it => <li key={it.id}><Link to={`/closet/${it.id}`}>{it.name}</Link>{it.status !== 'ready' && <span className="muted small">· {it.status}</span>}</li>)}</ul></Card>
      {wear !== null && <WearForm ids={o.item_ids} items={data.items} outfitId={o.id} planned={wear} date={wear ? new Date(Date.now() + 86400000).toISOString().slice(0, 10) : todayISO()} occasion={o.occasion} onClose={() => setWear(null)} />}
      {share && <ChatShare ids={o.item_ids} outfitId={o.id} title={o.name} onClose={() => setShare(false)} />}
      {del && <Confirm title="Delete this outfit?" message="The pieces stay in your closet; the days you wore it stay too." confirmLabel="Delete" onCancel={() => setDel(false)} onConfirm={async () => { await api.del(`${V1}/outfits/${o.id}`); announceChange(); navigate('/outfits') }} />}
    </div>
  )
}

/** Ideas: outfits from your own closet for a day, a style, an occasion. */
export function OutfitIdeas() {
  const cat = useCatalog()
  const { toast } = useToast()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const include = Number(params.get('include') || 0)
  const [occasion, setOccasion] = useState('everyday')
  const [styleId, setStyleId] = useState('')
  const [weather, setWeather] = useState<'today' | 'warm' | 'mild' | 'cold'>('today')
  const [shuffle, setShuffle] = useState(0)
  const [res, setRes] = useState<SuggestOut | null>(null)
  const [busy, setBusy] = useState(false)
  const [wear, setWear] = useState<number[] | null>(null)
  useEffect(() => {
    setBusy(true)
    const temps: Record<string, [number, number] | null> = { today: null, warm: [20, 29], mild: [9, 17], cold: [-3, 5] }
    const t = temps[weather]
    api.post<SuggestOut>(`${V1}/suggest`, { occasion, style: styleId, include, shuffle, n: 9, ...(t ? { t_min: t[0], t_max: t[1], season: weather === 'warm' ? 'summer' : weather === 'cold' ? 'winter' : 'autumn' } : {}) })
      .then(setRes).catch(e => toast(errMsg(e), 'err')).finally(() => setBusy(false))
  }, [occasion, styleId, weather, shuffle, include, toast])
  const save = async (s: Score) => {
    try { const o = await api.post<Outfit>(`${V1}/outfits`, { item_ids: s.items, occasion, style: styleId || res?.style, made_by: 'ideas' }); announceChange(); navigate(`/outfits/${o.id}`) } catch (e) { toast(errMsg(e), 'err') }
  }
  return (
    <div className="page">
      <div className="page-head row wrap">
        <div className="grow"><h2 className="page-title">Outfit ideas</h2><p className="muted">From what is in your closet now (not the laundry){include && res?.items[String(include)] ? `, each with ${res.items[String(include)].name}` : ''}.</p></div>
        <button type="button" className="btn" onClick={() => setShuffle(s => s + 1)} disabled={busy}>{busy ? <Spinner /> : <Icon name="shuffle" size={15} />}Others</button>
      </div>
      <div className="filters row wrap">
        <Select ariaLabel="Occasion" value={occasion} onChange={setOccasion} options={(cat?.occasions || []).map(o => ({ value: o.id, label: o.label }))} />
        <Select ariaLabel="Style" value={styleId} onChange={setStyleId} options={[{ value: '', label: 'Your style' }, { value: 'any', label: 'Any style' }, ...(cat?.styles || []).map(s => ({ value: s.id, label: s.name }))]} />
        <Select ariaLabel="Weather" value={weather} onChange={v => setWeather(v as typeof weather)} options={[{ value: 'today', label: "Today's weather" }, { value: 'warm', label: 'Warm (20–29°C)' }, { value: 'mild', label: 'Mild (9–17°C)' }, { value: 'cold', label: 'Cold (below 5°C)' }]} />
        {weather === 'today' && res && <WeatherChip w={res.weather} why={res.weather_why} />}
      </div>
      {!res ? <Loading /> : res.empty ? <Empty title="No ideas yet" icon="hanger"><p>{res.empty}</p></Empty>
        : <div className="idea-grid">
          {res.suggestions.map(s => <div key={s.items.join('-')} className="idea-card card">
            <OutfitBoard ids={s.items} items={res.items} size="md" />
            <div className="row tight"><ScoreBadge score={s.score} /><span className="grow" /><span className="color-dots">{s.colors.slice(0, 5).map(c => <span key={c} className="color-dot" title={c} style={{ background: cat?.families.find(f => f.name === c)?.hex }} />)}</span></div>
            <Reasons s={s} max={2} />
            <div className="row wrap tight"><button type="button" className="btn sm primary" onClick={() => setWear(s.items)}>Wear it</button><button type="button" className="btn sm" onClick={() => save(s)}>Save</button><Link className="btn sm ghost" to={`/outfits/new?items=${s.items.join(',')}`}>Change</Link></div>
          </div>)}
        </div>}
      {wear && res && <WearForm ids={wear} items={res.items} occasion={occasion} onClose={() => setWear(null)} />}
    </div>
  )
}
