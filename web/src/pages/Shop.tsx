// Worth buying: what is missing for your style (each drawn, with how many
// new outfits it makes and those outfits), the AI's ideas, "should I buy
// this?", the clothes you bought (Pocket Money, Pocket Parcels) and your
// clothing wishes (Pocket Wishlist).
import { useCallback, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useAppState, useOnChange } from '../lib/data'
import { fmtDate } from '../lib/format'
import type { AIPiece, Gap, Idea, Item, Kind, Purchase, Score, ShopLink } from '../lib/types'
import { FORMAL_WORDS, PATTERN_WORDS, kindLabel, money, shapeOf, styleName, useCatalog } from '../lib/wardrobe'
import { ItemForm, PhotoButton, type ItemInitial } from '../components/Forms'
import { Garment } from '../components/Garment'
import { ItemCard, OutfitBoard, Reasons, ShopLinks, Swatch } from '../components/Pieces'
import { AppOff } from '../components/Apps'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { Card, Empty, Field, Loading, Spinner, Tabs, errMsg, useAsync, useToast } from '../components/ui'

type TabId = 'ideas' | 'check' | 'bought' | 'wishes'

export default function Shop() {
  const { tab: raw } = useParams()
  const navigate = useNavigate()
  const tab = (['ideas', 'check', 'bought', 'wishes'].includes(raw || '') ? raw : 'ideas') as TabId
  return (
    <div className="page">
      <div className="page-head"><h2 className="page-title">Worth buying</h2><p className="muted">Fewer, better buys: what fills a gap in your wardrobe, and what you already have enough of.</p></div>
      <Tabs tabs={[{ id: 'ideas', label: 'Ideas', icon: 'sparkles' }, { id: 'check', label: 'Should I buy this?', icon: 'cart' }, { id: 'bought', label: 'Bought', icon: 'receipt' }, { id: 'wishes', label: 'Wishlist', icon: 'gift' }]} value={tab} onChange={t => navigate(`/shop/${t === 'ideas' ? '' : t}`, { replace: true })} />
      <div className="mt">
        {tab === 'ideas' && <Ideas />}
        {tab === 'check' && <Check />}
        {tab === 'bought' && <Bought />}
        {tab === 'wishes' && <Wishes />}
      </div>
    </div>
  )
}

interface GapRow { gap: Gap; status: string; idea_id: number; shops: ShopLink[]; price_hint?: string }

/** One thing worth buying, drawn, with the outfits it makes. */
function IdeaCard({ row, items, onKeep, kept }: { row: GapRow; items: Record<string, Item>; onKeep?: (r: GapRow, status: 'idea' | 'wish' | 'dismissed') => void; kept?: string }) {
  const cat = useCatalog()
  const g = row.gap
  const appState = useAppState()
  const wl = appState('wishlist', 'wishes:write')
  const accessory = g.category === 'accessory' || g.category === 'bag'
  return (
    <div className="card idea-big">
      <div className="ib-top">
        <span className="ib-art"><Garment shape={shapeOf(cat, g.kind)} color={g.color_hex} size={112} title={g.title} /></span>
        <div className="stack tight grow min0">
          <b className="ib-title">{g.title}</b>
          <span className="muted small">{kindLabel(cat, g.kind)} · {g.color_name}{g.style ? ` · ${styleName(cat, g.style)}` : ''}{row.price_hint ? ` · ${row.price_hint}` : ''}</span>
          <p className="small m0">{g.why}</p>
          {g.unlocks > 0 && <span className="unlock"><Icon name="layers" size={13} />{accessory ? `Finishes about ${g.unlocks} of your outfits` : `About ${g.unlocks} new outfits with what you have`}</span>}
        </div>
      </div>
      {g.examples.length > 0 && <div className="ib-examples">
        {g.examples.map((ex: Score) => <OutfitBoard key={ex.items.join('-')} ids={ex.items} items={items} size="sm" idea={{ kind: g.kind, color_hex: g.color_hex, title: g.title }} />)}
      </div>}
      <div className="row wrap tight">
        <ShopLinks shops={row.shops} />
        {onKeep && <span className="row tight ib-keep">
          {kept === 'wish' ? <span className="pill st-ok"><Icon name="gift" size={12} />On your wishlist</span>
            : <button type="button" className="btn sm" disabled={!wl.ok} title={wl.ok ? 'Put it on Pocket Wishlist' : `Pocket Wishlist: ${wl.app?.detail || 'not connected'}`} onClick={() => onKeep(row, 'wish')}><Icon name="gift" size={13} />Wishlist</button>}
          {kept !== 'idea' && kept !== 'wish' && <button type="button" className="btn sm ghost" onClick={() => onKeep(row, 'idea')}><Icon name="star" size={13} />Keep</button>}
          <button type="button" className="btn sm ghost" onClick={() => onKeep(row, 'dismissed')} title="Do not show this again">Not for me</button>
        </span>}
      </div>
    </div>
  )
}

function Ideas() {
  const { settings } = useApp()
  const cat = useCatalog()
  const { toast } = useToast()
  const [styleId, setStyleId] = useState(() => new URLSearchParams(window.location.search).get('style') || '')
  const { data, reload } = useAsync(() => api.get<{ ideas: GapRow[]; items: Record<string, Item>; style_names: string[]; pieces: number }>(`${V1}/gaps${styleId ? `?style=${styleId}` : ''}`), [styleId])
  const { data: kept, reload: reloadKept } = useAsync(() => api.get<{ ideas: { idea: Idea; shops: ShopLink[] }[] }>(`${V1}/ideas`), [])
  useOnChange(useCallback(() => { reload(); reloadKept() }, [reload, reloadKept]))
  const [ai, setAi] = useState<{ ideas: GapRow[]; items: Record<string, Item> } | null>(null)
  const [wish, setWish] = useState('')
  const [busy, setBusy] = useState(false)
  const keep = async (r: GapRow, status: 'idea' | 'wish' | 'dismissed') => {
    try {
      const g = r.gap
      let id = r.idea_id
      if (!id) id = (await api.post<Idea>(`${V1}/ideas`, { title: g.title, kind: g.kind, color_hex: g.color_hex, color_name: g.color_name, why: g.why, style: g.style, unlocks: g.unlocks, price_hint: r.price_hint || '', made_by: r.idea_id === -1 ? 'ai' : 'gaps', status: status === 'wish' ? 'idea' : status })).id
      else await api.put(`${V1}/ideas/${id}`, { status: status === 'wish' ? 'idea' : status })
      if (status === 'wish') { const res = await api.post<{ text: string }>(`${V1}/ideas/${id}/wishlist`); toast(res.text) } else toast(status === 'dismissed' ? 'Hidden' : 'Kept')
      announceChange()
    } catch (e) { toast(errMsg(e), 'err') }
  }
  const askAI = async () => {
    setBusy(true)
    try { const r = await api.post<{ ideas: GapRow[]; items: Record<string, Item> }>(`${V1}/ai/shopping`, { wish, budget: settings?.budget ? money(settings.budget, settings.currency) + ' a month' : '' }); setAi({ ideas: r.ideas.map(x => ({ ...x, status: '', idea_id: 0 })), items: r.items }) } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  if (!data || !cat) return <Loading />
  const keptList = (kept?.ideas || []).filter(k => k.idea.status === 'idea' || k.idea.status === 'wish')
  return (
    <div className="stack loose">
      <div className="row wrap">
        <p className="muted grow m0">{data.style_names.length ? `For ${data.style_names.join(' and ')}` : 'For smart casual (pick your style on Your style)'} — the pieces each style is built on that you do not have yet.</p>
        <Select ariaLabel="Style" value={styleId} onChange={setStyleId} options={[{ value: '', label: 'Your styles' }, ...cat.styles.map(s => ({ value: s.id, label: s.name }))]} />
      </div>
      {data.ideas.length === 0 ? <Card><Empty title={data.pieces < 5 ? 'Add a few pieces first' : 'Nothing missing'} icon="check-circle"><p>{data.pieces < 5 ? 'With a handful of pieces in the closet, the gaps show here.' : 'You have every staple of this style. Ask the AI below for fresh ideas, or try another style.'}</p></Empty></Card>
        : <div className="idea-list">{data.ideas.map(r => <IdeaCard key={r.gap.kind + r.gap.color_name} row={r} items={data.items} onKeep={keep} kept={r.status} />)}</div>}
      {settings?.ai_help && <Card title={<span className="row tight"><Icon name="sparkles" size={16} />Ideas from the AI</span>} sub="Six pieces that would make the most new outfits with yours.">
        <div className="row wrap"><input className="grow" value={wish} onChange={e => setWish(e.target.value)} placeholder="Optional: something for a wedding in May, under 150 €" onKeyDown={e => { if (e.key === 'Enter') askAI() }} /><button type="button" className="btn primary" onClick={askAI} disabled={busy}>{busy ? <Spinner /> : <Icon name="sparkles" size={15} />}Ask</button></div>
        {busy && <p className="muted small mt-s">The AI is looking at your wardrobe…</p>}
        {ai && <div className="idea-list mt">{ai.ideas.map(r => <IdeaCard key={r.gap.title} row={{ ...r, idea_id: 0 }} items={ai.items} onKeep={(row, st) => keep({ ...row, idea_id: 0 }, st)} />)}</div>}
      </Card>}
      {keptList.length > 0 && <Card title="Kept ideas">
        <ul className="list">{keptList.map(({ idea: i, shops }) => (
          <li key={i.id} className="list-row">
            <Garment shape={shapeOf(cat, i.kind)} color={i.color_hex || '#999'} size={44} />
            <div className="grow min0"><b className="ellipsis">{i.title}</b><div className="muted small">{i.unlocks ? `${i.unlocks} new outfits · ` : ''}{i.status === 'wish' ? <a href={i.wish_url} target="_blank" rel="noreferrer">on your wishlist</a> : 'kept'}</div></div>
            <ShopLinks shops={shops.slice(0, 1)} />
            <button type="button" className="btn sm" onClick={async () => { await api.post(`${V1}/ideas/${i.id}/bought`); announceChange(); toast('In your closet — add a photo when it arrives') }}>Bought it</button>
            <button type="button" className="btn sm ghost icon-only" aria-label="Remove" onClick={async () => { await api.del(`${V1}/ideas/${i.id}`); announceChange() }}><Icon name="x" size={14} /></button>
          </li>))}</ul>
      </Card>}
    </div>
  )
}

interface Verdict { verdict: 'great' | 'good' | 'think' | 'skip'; headline: string; reasons: string[]; similar: number[]; outfits: number; examples: Score[]; color_name: string }

/** "Should I buy this?": a photo, a shop link or a few words. */
function Check() {
  const { settings } = useApp()
  const cat = useCatalog()
  const { toast } = useToast()
  const [kind, setKind] = useState('sweater')
  const [hex, setHex] = useState('#22304f')
  const [pattern, setPattern] = useState('plain')
  const [link, setLink] = useState('')
  const [page, setPage] = useState<{ title: string; image: string; price?: number; currency?: string; shop?: string } | null>(null)
  const [photo, setPhoto] = useState<{ id: number; url: string } | null>(null)
  const [busy, setBusy] = useState('')
  const [res, setRes] = useState<{ verdict: Verdict; items: Record<string, Item>; kind: Kind; color_hex: string } | null>(null)
  const check = async (k = kind, h = hex, p = pattern) => {
    setBusy('check')
    try { setRes(await api.post(`${V1}/check`, { kind: k, color_hex: h, pattern: p })) } catch (e) { toast(errMsg(e), 'err') } finally { setBusy('') }
  }
  const fromAI = async (body: Record<string, unknown>) => {
    setBusy('ai')
    try {
      const r = await api.post<{ item: AIPiece }>(`${V1}/ai/check`, body)
      const k = r.item.kind, h = r.item.colors[0]?.hex || hex, p = r.item.pattern
      setKind(k); setHex(h); setPattern(p)
      await check(k, h, p)
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy('') }
  }
  const readLink = async () => {
    setBusy('link')
    try { const r = await api.post<{ title: string; image: string; price?: number; currency?: string; shop?: string }>(`${V1}/preview`, { url: link }); setPage(r); if (settings?.ai_help) await fromAI({ title: r.title }) } catch (e) { toast(errMsg(e), 'err') } finally { setBusy('') }
  }
  const aiOn = settings?.ai_help && settings?.ai_photos
  const v = res?.verdict
  const tone = v ? { great: 'good', good: 'good', think: 'warn', skip: 'bad' }[v.verdict] : ''
  return (
    <div className="check-grid">
      <div className="stack loose min0">
        <Card title="Show it" sub="A photo in the shop, a link, or say what it is.">
          <div className="stack">
            <div className="row wrap">
              {aiOn ? <PhotoButton kind="check" capture="environment" label="Photo of it" onPicked={(_, p) => { setPhoto(p); fromAI({ photo_id: p.id }) }} /> : <span className="muted small">Photos need the AI (Settings → AI).</span>}
              {busy === 'ai' && <span className="muted small"><Spinner /> Reading it…</span>}
            </div>
            <Field label="Or a shop link"><div className="row tight"><input className="grow" value={link} onChange={e => setLink(e.target.value)} placeholder="https://…" /><button type="button" className="btn" onClick={readLink} disabled={!link.trim() || !!busy}>{busy === 'link' ? <Spinner /> : 'Read'}</button></div></Field>
            {page && <div className="row tight shop-page">{page.image && <img src={page.image} alt="" />}<span className="small"><b>{page.title}</b>{page.price != null && <> · {money(page.price, page.currency || 'EUR')}</>}{page.shop && <span className="muted"> · {page.shop}</span>}</span></div>}
            <div className="two-col tight">
              <Field label="What it is"><Select ariaLabel="Kind" value={kind} onChange={setKind} options={(cat?.kinds || []).map(k => ({ value: k.id, label: k.label }))} /></Field>
              <Field label="Pattern"><Select ariaLabel="Pattern" value={pattern} onChange={setPattern} options={Object.entries(PATTERN_WORDS).map(([value, label]) => ({ value, label }))} /></Field>
            </div>
            <Field label="Colour"><div className="row wrap tight">{(cat?.families || []).filter(f => f.name !== 'gold' && f.name !== 'silver').map(f => <Swatch key={f.name} hex={f.hex} name={f.name} size={26} on={hex === f.hex} onClick={() => setHex(f.hex)} />)}<label className="swatch-btn custom" title="Exact colour" style={{ background: hex }}><input type="color" value={hex} onChange={e => setHex(e.target.value)} /></label></div></Field>
            <div className="row"><button type="button" className="btn primary" onClick={() => check()} disabled={!!busy}>{busy === 'check' ? <Spinner /> : <Icon name="check" size={15} />}Check it</button></div>
          </div>
        </Card>
      </div>
      <div className="stack loose min0">
        {!v ? <Card><Empty title="How it would fit in" icon="cart"><p>See how many outfits it makes with what you have, and whether you own something much like it already.</p></Empty></Card>
          : <Card className={`verdict ${tone}`}>
            <div className="row tight">
              {photo ? <img className="verdict-pic" src={photo.url} alt="" /> : <Garment shape={res!.kind.shape} color={res!.color_hex} pattern={pattern} size={88} />}
              <div className="grow"><h3 className="m0">{v.headline}</h3><p className="muted small m0">{res!.kind.label} · {v.color_name} · {FORMAL_WORDS[res!.kind.formality]}</p></div>
            </div>
            <ul className="reasons mt">{v.reasons.map(r => <li key={r} className={v.verdict === 'skip' ? 'warn' : 'good'}><Icon name={v.verdict === 'skip' ? 'alert' : 'check'} size={13} />{r}</li>)}</ul>
            {v.similar.length > 0 && <><h4 className="sec-title">You already have</h4><div className="item-grid small">{v.similar.map(id => res!.items[String(id)]).filter(Boolean).map(it => <ItemCard key={it.id} item={it} to={`/closet/${it.id}`} />)}</div></>}
            {v.examples.length > 0 && <><h4 className="sec-title">Outfits it would make</h4><div className="ib-examples">{v.examples.map(ex => <div key={ex.items.join('-')} className="stack tight"><OutfitBoard ids={ex.items} items={res!.items} size="sm" idea={{ kind: res!.kind.id, color_hex: res!.color_hex, title: res!.kind.label }} /><Reasons s={ex} max={1} /></div>)}</div></>}
            <BuyActions kind={res!.kind} hex={res!.color_hex} pattern={pattern} title={page?.title} url={link} photo={photo} price={page?.price} unlocks={v.outfits} />
          </Card>}
      </div>
    </div>
  )
}

function BuyActions({ kind, hex, pattern, title, url, photo, price, unlocks }: { kind: Kind; hex: string; pattern: string; title?: string; url: string; photo: { id: number; url: string } | null; price?: number; unlocks: number }) {
  const { toast } = useToast()
  const appState = useAppState()
  const wl = appState('wishlist', 'wishes:write')
  const [add, setAdd] = useState<ItemInitial | null>(null)
  const name = title || kind.label
  const keep = async (wish: boolean) => {
    try {
      const i = await api.post<Idea>(`${V1}/ideas`, { title: name, kind: kind.id, color_hex: hex, unlocks, url, photo_id: photo?.id || 0, made_by: 'check' })
      if (wish) { const r = await api.post<{ text: string }>(`${V1}/ideas/${i.id}/wishlist`); toast(r.text) } else toast('Kept in your ideas')
      announceChange()
    } catch (e) { toast(errMsg(e), 'err') }
  }
  return (
    <div className="row wrap mt">
      <button type="button" className="btn primary" onClick={() => setAdd({ name, kind: kind.id, category: kind.category, colors: [{ hex, name: '' }], pattern, warmth: kind.warmth, formality: kind.formality, photo_id: photo?.id || null, photo_url: photo?.url, price_text: price != null ? (price / 100).toFixed(2) : '' })}><Icon name="check" size={15} />I bought it</button>
      <button type="button" className="btn" disabled={!wl.ok} title={wl.ok ? 'Put it on Pocket Wishlist' : `Pocket Wishlist: ${wl.app?.detail || 'not connected'}`} onClick={() => keep(true)}><Icon name="gift" size={15} />Wishlist</button>
      <button type="button" className="btn ghost" onClick={() => keep(false)}>Keep the idea</button>
      {add && <ItemForm initial={add} onClose={() => setAdd(null)} />}
    </div>
  )
}

/** Clothes bought (Pocket Money) and on their way (Pocket Parcels). */
function Bought() {
  const appState = useAppState()
  const money_ = appState('money', 'transactions:read')
  const parcels = appState('parcels', 'parcels:read')
  const { data, reload } = useAsync(() => api.get<{ purchases: Purchase[]; errors: Record<string, string> }>(`${V1}/purchases`), [])
  useOnChange(useCallback(() => { reload() }, [reload]))
  const [add, setAdd] = useState<ItemInitial | null>(null)
  if (!data) return <Loading />
  return (
    <div className="stack loose">
      <p className="muted">Clothes and shoes from your spending in Pocket Money and your deliveries in Pocket Parcels (by shop name and words like “jacket”). Put them in the closet with their price, for cost per wear.</p>
      {!money_.ok && !money_.loading && <AppOff slug="money" app={money_.app}>Clothes you paid for show here.</AppOff>}
      {!parcels.ok && !parcels.loading && <AppOff slug="parcels" app={parcels.app}>Clothes on their way, and their return deadlines, show here.</AppOff>}
      {data.purchases.length === 0 ? <Card><Empty title="No clothes bought lately" icon="receipt" /></Card>
        : <Card flush><ul className="list">{data.purchases.map(p => (
          <li key={p.app + p.ref} className="list-row">
            <span className="list-ico"><Icon name={p.app === 'money' ? 'wallet' : 'package'} size={16} /></span>
            <div className="grow min0">
              <b className="ellipsis">{p.title}</b>
              <div className="muted small">{[p.shop !== p.title && p.shop, p.date && fmtDate(p.date), p.amount != null && money(p.amount, p.currency), p.status === 'in_transit' || p.status === 'out_for_delivery' ? `on its way${p.expected_on ? ', ' + fmtDate(p.expected_on) : ''}` : '', p.return_by ? `return by ${fmtDate(p.return_by)}` : ''].filter(Boolean).join(' · ')}</div>
            </div>
            {p.url && <a className="btn sm ghost" href={p.url} target="_blank" rel="noreferrer">Open</a>}
            {p.added ? <Link className="btn sm" to={`/closet/${p.added}`}>In the closet</Link>
              : <button type="button" className="btn sm primary" onClick={() => setAdd({ name: '', shop: p.shop, bought_on: p.date, price_text: p.amount != null ? (p.amount / 100).toFixed(2) : '', currency: p.currency || undefined, source: p.app, source_ref: p.ref })}><Icon name="plus" size={13} />Add</button>}
          </li>))}</ul></Card>}
      {add && <ItemForm initial={add} onClose={() => setAdd(null)} onSaved={() => reload()} />}
    </div>
  )
}

/** Clothing wishes in Pocket Wishlist, each one a click from "should I buy this?". */
function Wishes() {
  const appState = useAppState()
  const wl = appState('wishlist', 'wishes:read')
  const { data, error } = useAsync(() => wl.ok || wl.loading ? api.get<{ wishes: { ref: string; title: string; store: string; price: number | null; currency: string; image: string; url: string; open_url?: string }[] }>(`${V1}/wishes`) : Promise.resolve({ wishes: [] }), [wl.ok])
  if (!wl.ok && !wl.loading) return <AppOff slug="wishlist" app={wl.app}>Your clothing wishes show here, each with “how would it fit in?”.</AppOff>
  if (error) return <p className="muted">{errMsg(error)}</p>
  if (!data) return <Loading />
  return data.wishes.length === 0 ? <Card><Empty title="No clothing wishes" icon="gift"><p>Ideas you send to Pocket Wishlist show here.</p></Empty></Card>
    : <Card flush><ul className="list">{data.wishes.map(w => (
      <li key={w.ref} className="list-row">
        {w.image ? <img className="wish-pic" src={w.image} alt="" /> : <span className="list-ico"><Icon name="gift" size={16} /></span>}
        <div className="grow min0"><b className="ellipsis">{w.title}</b><div className="muted small">{[w.store, w.price != null && money(w.price, w.currency)].filter(Boolean).join(' · ')}</div></div>
        {w.url && <a className="btn sm ghost" href={w.url} target="_blank" rel="noreferrer">Shop</a>}
        <Link className="btn sm" to="/shop/check">Check it</Link>
      </li>))}</ul></Card>
}
