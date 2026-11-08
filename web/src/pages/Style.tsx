// Your style: the ways you like to dress, the colours that suit you (the
// AI can read them from a photo), sizes and fit, and "rate my look".
import { useState } from 'react'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange } from '../lib/data'
import type { Item, Settings, StyleDef } from '../lib/types'
import { familyHex, useCatalog } from '../lib/wardrobe'
import { PhotoButton } from '../components/Forms'
import { Garment } from '../components/Garment'
import { ItemThumb, Swatch } from '../components/Pieces'
import { Icon } from '../components/icons'
import { Card, Field, Loading, Spinner, errMsg, useToast } from '../components/ui'
import { Select } from '../components/Select'

export default function StylePage() {
  const { settings, setSettings } = useApp()
  const cat = useCatalog()
  const { toast } = useToast()
  if (!settings || !cat) return <Loading />
  const save = async (patch: Partial<Settings>, msg = 'Saved') => {
    try { const s = await api.put<Settings>(`${V1}/settings`, patch); setSettings(s); announceChange(); toast(msg) } catch (e) { toast(errMsg(e), 'err') }
  }
  const toggleStyle = (id: string) => {
    const cur = settings.styles
    save({ styles: cur.includes(id) ? cur.filter(x => x !== id) : [...cur, id].slice(-4) })
  }
  const toggleColor = (key: 'love_colors' | 'avoid_colors', name: string) => {
    const cur = settings[key]
    save({ [key]: cur.includes(name) ? cur.filter(x => x !== name) : [...cur, name] } as Partial<Settings>)
  }
  return (
    <div className="page">
      <div className="page-head"><h2 className="page-title">Your style</h2><p className="muted">Pick how you like to dress; every idea, every outfit score and every shopping tip follows it.</p></div>
      <Card title="How you like to dress" sub="Pick one to four. The first is the one the ideas aim for unless you choose another.">
        <div className="style-grid">
          {cat.styles.map(s => <StyleCard key={s.id} s={s} on={settings.styles.includes(s.id)} first={settings.styles[0] === s.id} onClick={() => toggleStyle(s.id)} />)}
        </div>
      </Card>
      <FindStyle onUse={(styles, palette) => save({ styles, love_colors: palette }, 'Your style is set')} />
      <Card title="Colours" sub="Colours you love are favoured in ideas; colours you avoid are left out.">
        <div className="stack">
          <Field label="I love"><div className="row wrap tight">{cat.families.filter(f => f.name !== 'gold' && f.name !== 'silver').map(f => <Swatch key={f.name} hex={f.hex} name={f.name} on={settings.love_colors.includes(f.name)} onClick={() => toggleColor('love_colors', f.name)} />)}</div></Field>
          <Field label="I avoid"><div className="row wrap tight">{cat.families.filter(f => f.name !== 'gold' && f.name !== 'silver').map(f => <Swatch key={f.name} hex={f.hex} name={f.name} on={settings.avoid_colors.includes(f.name)} onClick={() => toggleColor('avoid_colors', f.name)} />)}</div></Field>
        </div>
      </Card>
      <Colours onUse={(season, best, avoid, notes) => save({ color_season: season, love_colors: best, avoid_colors: avoid, color_notes: notes }, 'Your colours are set')} />
      <RateLook />
      <Profile s={settings} save={save} />
    </div>
  )
}

function StyleCard({ s, on, first, onClick }: { s: StyleDef; on: boolean; first: boolean; onClick: () => void }) {
  const cat = useCatalog()
  const staples = s.staples.slice(0, 3)
  return (
    <button type="button" className={`style-card ${on ? 'on' : ''}`} onClick={onClick} aria-pressed={on}>
      <span className="sc-art">{staples.map(st => <Garment key={st.kind + st.family} shape={cat?.kinds.find(k => k.id === st.kind)?.shape || 'tshirt'} color={familyHex(cat, st.family)} size={54} />)}</span>
      <span className="sc-name">{s.name}{first && <span className="badge accent">main</span>}{on && !first && <Icon name="check" size={14} />}</span>
      <span className="sc-desc">{s.description}</span>
      <span className="sc-palette">{s.palette.slice(0, 7).map(f => <span key={f} className="color-dot" style={{ background: familyHex(cat, f) }} title={f} />)}</span>
    </button>
  )
}

function FindStyle({ onUse }: { onUse: (styles: string[], palette: string[]) => void }) {
  const { settings } = useApp()
  const cat = useCatalog()
  const { toast } = useToast()
  const [about, setAbout] = useState('')
  const [busy, setBusy] = useState(false)
  const [res, setRes] = useState<{ summary: string; styles: string[]; palette: string[]; strengths: string[]; try: string[]; let_go: string[] } | null>(null)
  if (!settings?.ai_help) return null
  const run = async () => { setBusy(true); try { setRes(await api.post(`${V1}/ai/find-style`, { about })) } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) } }
  return (
    <Card title={<span className="row tight"><Icon name="sparkles" size={16} />Find my style</span>} sub="The AI looks at what you own and what you actually wear.">
      <div className="row wrap"><input className="grow" value={about} onChange={e => setAbout(e.target.value)} placeholder="Optional: I work in an office, cycle to work, like to look neat but relaxed" /><button type="button" className="btn primary" onClick={run} disabled={busy}>{busy ? <Spinner /> : <Icon name="sparkles" size={15} />}Find it</button></div>
      {res && <div className="ai-box stack mt">
        <p>{res.summary}</p>
        <div className="row wrap tight">{res.styles.map(s => <span key={s} className="chip static">{cat?.styles.find(x => x.id === s)?.name || s}</span>)}{res.palette.map(f => <span key={f} className="color-dot lg" title={f} style={{ background: familyHex(cat, f) }} />)}</div>
        {res.strengths.length > 0 && <div><b className="small">Works well</b><ul className="tips">{res.strengths.map(t => <li key={t}><Icon name="check" size={14} />{t}</li>)}</ul></div>}
        {res.try.length > 0 && <div><b className="small">Try</b><ul className="tips">{res.try.map(t => <li key={t}><Icon name="sparkles" size={14} />{t}</li>)}</ul></div>}
        {res.let_go.length > 0 && <div><b className="small">Could let go</b><ul className="tips">{res.let_go.map(t => <li key={t}><Icon name="archive" size={14} />{t}</li>)}</ul></div>}
        <div className="row"><button type="button" className="btn" onClick={() => onUse(res.styles, res.palette)}>Use this style and these colours</button></div>
      </div>}
    </Card>
  )
}

function Colours({ onUse }: { onUse: (season: string, best: string[], avoid: string[], notes: string) => void }) {
  const { settings } = useApp()
  const cat = useCatalog()
  const { toast } = useToast()
  const [busy, setBusy] = useState(false)
  const [res, setRes] = useState<{ season: string; undertone: string; summary: string; best: string[]; avoid: string[]; metals: string; confidence: string } | null>(null)
  if (!settings) return null
  const run = async (id: number) => { setBusy(true); try { setRes(await api.post(`${V1}/ai/colours`, { photo_id: id })) } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) } }
  const aiOn = settings.ai_help && settings.ai_photos
  return (
    <Card title={<span className="row tight"><Icon name="palette" size={16} />Colours that suit you</span>} sub={settings.color_season ? `Your colour season: ${settings.color_season}${settings.color_notes ? ' — ' + settings.color_notes : ''}` : 'A photo of your face in daylight, no make-up: the AI says which colours flatter you. The photo is not kept.'}>
      {aiOn ? <div className="row wrap"><PhotoButton kind="check" capture="user" label={busy ? 'Looking…' : 'Take a selfie'} onPicked={(_, p) => run(p.id)} />{busy && <Spinner />}</div>
        : <p className="muted small">Needs the AI with photos switched on (Settings → AI).</p>}
      {res && <div className="ai-box stack mt">
        <p><b>{res.season ? res.season[0].toUpperCase() + res.season.slice(1) : ''}</b>{res.undertone && ` · ${res.undertone} undertone`}{res.metals && ` · ${res.metals}`}{res.confidence && <span className="muted"> · {res.confidence} confidence</span>}</p>
        <p>{res.summary}</p>
        <div className="row wrap tight"><span className="small">Best:</span>{res.best.map(f => <span key={f} className="color-dot lg" title={f} style={{ background: familyHex(cat, f) }} />)}</div>
        <div className="row wrap tight"><span className="small">Avoid:</span>{res.avoid.map(f => <span key={f} className="color-dot lg" title={f} style={{ background: familyHex(cat, f) }} />)}</div>
        <div className="row"><button type="button" className="btn" onClick={() => onUse(res.season, res.best, res.avoid, `${res.undertone} undertone; ${res.metals}`)}>Use these colours</button></div>
      </div>}
    </Card>
  )
}

/** A mirror photo; the AI says what works and what to swap from your closet. */
function RateLook() {
  const { settings } = useApp()
  const cat = useCatalog()
  const { toast } = useToast()
  const [occasion, setOccasion] = useState('everyday')
  const [question, setQuestion] = useState('')
  const [photo, setPhoto] = useState<{ id: number; url: string } | null>(null)
  const [busy, setBusy] = useState(false)
  const [res, setRes] = useState<{ score: number; headline: string; good: string[]; change: string[]; swaps: { instead_of: string; why: string; item?: Item }[]; occasion_fit: string } | null>(null)
  if (!settings) return null
  const aiOn = settings.ai_help && settings.ai_photos
  const run = async (id = photo?.id) => { if (!id) return; setBusy(true); try { setRes(await api.post(`${V1}/ai/rate-look`, { photo_id: id, occasion, question })) } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) } }
  return (
    <Card title={<span className="row tight" id="look"><Icon name="mirror" size={16} />Rate my look</span>} sub="A mirror photo of what you are wearing: what works, what to change, and what in your closet would be better.">
      {!aiOn ? <p className="muted small">Needs the AI with photos switched on (Settings → AI).</p> : <div className="stack">
        <div className="row wrap">
          <Select ariaLabel="For" value={occasion} onChange={setOccasion} options={(cat?.occasions || []).map(o => ({ value: o.id, label: o.label }))} />
          <input className="grow" value={question} onChange={e => setQuestion(e.target.value)} placeholder="Optional: does the jacket work with these shoes?" />
          <PhotoButton kind="look" capture="user" label={photo ? 'Another photo' : 'Mirror photo'} onPicked={(_, p) => { setPhoto(p); run(p.id) }} />
        </div>
        {busy && <p className="muted small"><Spinner /> Looking at your outfit…</p>}
        {res && <div className="look-res">
          {photo && <img src={photo.url} alt="" />}
          <div className="stack">
            {res.score > 0 && <div className="look-score"><b>{res.score}</b>/10</div>}
            <p><b>{res.headline}</b></p>
            {res.occasion_fit && <p className="muted small">{res.occasion_fit}</p>}
            <ul className="reasons">{res.good.map(g => <li key={g} className="good"><Icon name="check" size={13} />{g}</li>)}{res.change.map(g => <li key={g} className="warn"><Icon name="edit" size={13} />{g}</li>)}</ul>
            {res.swaps.map((s, i) => <div key={i} className="swap">{s.item && <ItemThumb item={s.item} size={48} />}<span className="small">{s.item ? <>Swap <b>{s.instead_of}</b> for your <b>{s.item.name}</b>: </> : <><b>{s.instead_of}</b>: </>}{s.why}</span></div>)}
          </div>
        </div>}
      </div>}
    </Card>
  )
}

function Profile({ s, save }: { s: Settings; save: (p: Partial<Settings>) => void }) {
  const [f, setF] = useState({ presentation: s.presentation, size_top: s.size_top, size_bottom: s.size_bottom, size_shoe: s.size_shoe, fit_notes: s.fit_notes, budget: s.budget ? String(s.budget / 100) : '', currency: s.currency })
  return (
    <Card title="Sizes and fit" sub="For shopping searches and the AI's advice.">
      <div className="stack">
        <Field label="You wear"><Select ariaLabel="You wear" value={f.presentation} onChange={v => setF({ ...f, presentation: v as Settings['presentation'] })} options={[{ value: 'any', label: 'Anything' }, { value: 'menswear', label: 'Menswear' }, { value: 'womenswear', label: 'Womenswear' }]} /></Field>
        <div className="three-col tight">
          <Field label="Tops"><input value={f.size_top} onChange={e => setF({ ...f, size_top: e.target.value })} placeholder="M" /></Field>
          <Field label="Bottoms"><input value={f.size_bottom} onChange={e => setF({ ...f, size_bottom: e.target.value })} placeholder="32/32" /></Field>
          <Field label="Shoes"><input value={f.size_shoe} onChange={e => setF({ ...f, size_shoe: e.target.value })} placeholder="43" /></Field>
        </div>
        <Field label="What suits you" help="Fits you like, what you like to show or hide."><textarea rows={2} value={f.fit_notes} onChange={e => setF({ ...f, fit_notes: e.target.value })} /></Field>
        <div className="two-col tight">
          <Field label="Clothes budget a month"><input inputMode="decimal" value={f.budget} onChange={e => setF({ ...f, budget: e.target.value })} placeholder="none" /></Field>
          <Field label="Currency"><input value={f.currency} onChange={e => setF({ ...f, currency: e.target.value.toUpperCase().slice(0, 3) })} /></Field>
        </div>
        <div className="row"><button type="button" className="btn primary" onClick={() => save({ ...f, budget: f.budget ? Math.round(parseFloat(f.budget.replace(',', '.')) * 100) : 0 })}>Save</button></div>
      </div>
    </Card>
  )
}
