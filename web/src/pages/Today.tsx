// Today: the weather, what is on, and what to wear for it — one outfit
// big, others to swap to, the plan if there is one, and the AI's ideas.
import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useAppState, useOnChange } from '../lib/data'
import { fmtDate, todayISO } from '../lib/format'
import type { DayPlan, Item, Score, Wear } from '../lib/types'
import { occasionLabel, styleName, tempWords, useCatalog, weatherIcon } from '../lib/wardrobe'
import { openCreate } from '../components/Layout'
import { ChatShare, WearForm } from '../components/Forms'
import { ItemThumb, OutfitBoard, Reasons, ScoreBadge, WeatherChip } from '../components/Pieces'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { Card, Empty, Loading, Spinner, errMsg, useToast } from '../components/ui'

interface TodayOut { day: DayPlan; tomorrow?: { date: string; weather: DayPlan['weather']; plan: Wear | null }; laundry?: number; nudge?: Item | null }

export default function Today() {
  const { user, settings } = useApp()
  const cat = useCatalog()
  const navigate = useNavigate()
  const { toast } = useToast()
  const [params] = useSearchParams()
  const date = params.get('date') || todayISO()
  const [occasion, setOccasion] = useState('')
  const [styleId, setStyleId] = useState('')
  const [shuffle, setShuffle] = useState(0)
  const [pick, setPick] = useState(0)
  const [data, setData] = useState<TodayOut | null>(null)
  const [busy, setBusy] = useState(false)
  const [wear, setWear] = useState<{ ids: number[]; planned: boolean; date: string } | null>(null)
  const [share, setShare] = useState<number[] | null>(null)
  const load = useCallback(async () => {
    setBusy(true)
    try {
      const q = new URLSearchParams({ date, shuffle: String(shuffle) })
      if (occasion) q.set('occasion', occasion)
      if (styleId) q.set('style', styleId)
      setData(await api.get<TodayOut>(`${V1}/today?${q}`))
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }, [date, occasion, styleId, shuffle, toast])
  useEffect(() => { load() }, [load])
  useOnChange(load)
  useEffect(() => setPick(0), [shuffle, occasion, styleId])
  if (!data) return <Loading />
  const d = data.day
  const sugg = d.suggest.suggestions
  const cur: Score | undefined = sugg[pick]
  const hour = new Date().getHours()
  const hello = date !== todayISO() ? fmtDate(date) : `${hour < 12 ? 'Good morning' : hour < 18 ? 'Good afternoon' : 'Good evening'}${user?.name ? ', ' + user.name.split(' ')[0] : ''}`
  const occ = occasion || d.occasion
  const save = async (s: Score) => {
    try { const o = await api.post<{ id: number }>(`${V1}/outfits`, { item_ids: s.items, occasion: occ, style: d.suggest.style, made_by: 'ideas' }); announceChange(); toast('Saved as an outfit'); navigate(`/outfits/${o.id}`) } catch (e) { toast(errMsg(e), 'err') }
  }
  const wearPlan = async (p: Wear) => {
    try { await api.put(`${V1}/wears/${p.id}`, { planned: false }); announceChange(); toast('Logged as worn today') } catch (e) { toast(errMsg(e), 'err') }
  }
  const tomorrow = data.tomorrow
  return (
    <div className="page today">
      <div className="today-head">
        <div className="grow min0">
          <h2 className="page-title">{hello}</h2>
          <div className="row wrap tight mt-s">
            <WeatherChip w={d.weather} why={d.weather_why} />
            {d.because && <span className="chip static"><Icon name="calendar" size={13} />{d.because}</span>}
          </div>
        </div>
      </div>

      <div className="today-pick row wrap">
        <div className="chips scroll-x grow min0" role="radiogroup" aria-label="Dress for">
          {(cat?.occasions || []).filter(o => ['everyday', 'work', 'formal_work', 'date', 'dinner', 'party', 'wedding', 'sport', 'outdoors', 'travel', 'home'].includes(o.id)).map(o => (
            <button key={o.id} type="button" role="radio" aria-checked={occ === o.id} className={`chip ${occ === o.id ? 'on' : ''}`} onClick={() => setOccasion(o.id)}><Icon name={o.icon} size={13} />{o.label}</button>
          ))}
        </div>
        <Select ariaLabel="Style" value={styleId || d.suggest.style || ''} onChange={setStyleId} options={[{ value: 'any', label: 'Any style' }, ...(cat?.styles || []).map(s => ({ value: s.id, label: s.name + (settings?.styles.includes(s.id) ? ' ★' : '') }))]} />
      </div>

      <div className="today-grid">
        <div className="stack loose min0">
          {d.worn && <Card title="You wore today" actions={<Link className="btn sm ghost" to="/plan">Plan and history</Link>}>
            <OutfitBoard ids={d.worn.item_ids} items={d.items} size="md" />
          </Card>}
          {!d.worn && d.plan && <Card title={date === todayISO() ? 'Planned for today' : 'Planned'} actions={date === todayISO() && <button type="button" className="btn primary sm" onClick={() => wearPlan(d.plan!)}><Icon name="check" size={14} />I'm wearing it</button>}>
            <OutfitBoard ids={d.plan.item_ids} items={d.items} size="md" />
            {d.plan.event && <p className="muted small mt-s"><Icon name="calendar" size={13} className="inline-ico" /> For {d.plan.event}</p>}
          </Card>}

          {d.suggest.empty ? <Card><Empty title="Not enough to put together yet" icon="hanger"><p>{d.suggest.empty}</p><button type="button" className="btn primary" onClick={() => openCreate('item')}><Icon name="camera" size={15} />Add a piece</button></Empty></Card>
            : cur && <Card className="hero-card" title={d.plan || d.worn ? 'Or wear this' : `For ${occasionLabel(cat, occ).toLowerCase()}`} sub={d.suggest.style ? styleName(cat, d.suggest.style) + (d.suggest.feel ? ` · dressed for ${Math.round(d.suggest.feel)}°C` : '') : undefined}
              actions={<button type="button" className="btn sm" onClick={() => setShuffle(s => s + 1)} disabled={busy} title="Other ideas">{busy ? <Spinner /> : <Icon name="shuffle" size={14} />}Others</button>}>
              <div className="hero">
                <OutfitBoard ids={cur.items} items={d.items} size="lg" />
                <div className="hero-side">
                  <ScoreBadge score={cur.score} />
                  <Reasons s={cur} />
                  <div className="row wrap">
                    <button type="button" className="btn primary" onClick={() => setWear({ ids: cur.items, planned: false, date })}><Icon name="check" size={15} />{date === todayISO() ? 'Wear this today' : 'Wear this'}</button>
                    {date === todayISO() && <button type="button" className="btn" onClick={() => setWear({ ids: cur.items, planned: true, date: tomorrow?.date || date })}><Icon name="calendar-plus" size={15} />Tomorrow</button>}
                    <button type="button" className="btn" onClick={() => save(cur)}><Icon name="heart" size={15} />Save</button>
                    <ShareButton onShare={() => setShare(cur.items)} />
                  </div>
                </div>
              </div>
              {sugg.length > 1 && <div className="alt-strip">
                {sugg.map((s, i) => <button key={s.items.join('-')} type="button" className={`alt ${i === pick ? 'on' : ''}`} onClick={() => setPick(i)} aria-pressed={i === pick} title={`${Math.round(s.score)} out of 100`}>
                  <OutfitBoard ids={s.items} items={d.items} size="sm" />
                </button>)}
              </div>}
            </Card>}

          <StyleMe date={date} occasion={occ} styleId={styleId || d.suggest.style} onWear={ids => setWear({ ids, planned: false, date })} onSave={ids => save({ items: ids } as Score)} />
        </div>

        <div className="stack loose min0">
          {(d.events.length > 0 || d.events_why) && <Card title="On the calendar">
            {d.events.length ? <ul className="mini-list">{d.events.map(e => (
              <li key={e.ref}><Icon name="calendar" size={14} /><span className="grow ellipsis">{e.time || 'All day'} · {e.title}</span>{e.occasion && <button type="button" className={`chip sm ${occ === e.occasion ? 'on' : ''}`} onClick={() => setOccasion(e.occasion)}>{occasionLabel(cat, e.occasion)}</button>}</li>
            ))}</ul> : <p className="muted small">{d.events_why}</p>}
          </Card>}
          {d.tips.length > 0 && <Card title="Worth knowing"><ul className="tips">{d.tips.map(t => <li key={t}><Icon name="info" size={14} />{t}</li>)}</ul></Card>}
          {tomorrow && <Card title="Tomorrow" sub={tomorrow.weather ? `${tempWords(tomorrow.weather)} · ${tomorrow.weather.condition.toLowerCase()}` : undefined} actions={tomorrow.weather && <Icon name={weatherIcon(tomorrow.weather.code)} size={18} />}>
            {tomorrow.plan ? <><OutfitBoard ids={tomorrow.plan.item_ids} items={d.items} size="sm" />{tomorrow.plan.event && <p className="muted small">For {tomorrow.plan.event}</p>}</>
              : <div className="row wrap"><p className="muted small grow">Nothing planned yet.</p><Link className="btn sm" to={`/?date=${tomorrow.date}`}>Plan tomorrow</Link></div>}
          </Card>}
          {!!data.laundry && <Card title="Laundry basket" actions={<Link className="btn sm" to="/laundry">Open</Link>}><p className="muted small">{data.laundry} pieces waiting to be washed — they are left out of the ideas.</p></Card>}
          {data.nudge && <Card title="Waiting to be worn">
            <Link to={`/closet/${data.nudge.id}`} className="nudge"><ItemThumb item={data.nudge} size={64} /><span className="grow"><b>{data.nudge.name}</b><span className="muted small block-line">{data.nudge.last_worn ? `Last worn ${fmtDate(data.nudge.last_worn)}` : 'Not worn yet'}. Make it today's.</span></span></Link>
          </Card>}
        </div>
      </div>
      {wear && <WearForm ids={wear.ids} items={d.items} date={wear.date} planned={wear.planned} occasion={occ} event={d.because || ''} onClose={() => setWear(null)} />}
      {share && <ChatShare ids={share} onClose={() => setShare(null)} />}
    </div>
  )
}

function ShareButton({ onShare }: { onShare: () => void }) {
  const appState = useAppState()
  const chat = appState('chat', 'chat:write')
  return <button type="button" className="btn" onClick={onShare} disabled={!chat.ok} title={chat.ok ? 'Ask someone what they think' : `Pocket Chat: ${chat.app?.detail || 'not connected'}`}><Icon name="message" size={15} />Ask</button>
}

/** "Style me": the AI puts outfits together for a wish. */
export function StyleMe({ date, occasion, styleId, itemId, onWear, onSave }: { date: string; occasion: string; styleId: string; itemId?: number; onWear: (ids: number[]) => void; onSave: (ids: number[]) => void }) {
  const { settings } = useApp()
  const { toast } = useToast()
  const [wish, setWish] = useState('')
  const [busy, setBusy] = useState(false)
  const [res, setRes] = useState<{ outfits: { name: string; item_ids: number[]; why: string; rating: Score }[]; tip: string; items: Record<string, Item> } | null>(null)
  if (!settings?.ai_help) return null
  const ask = async () => {
    setBusy(true)
    try { setRes(await api.post(`${V1}/ai/style-me`, { date, occasion, style: styleId === 'any' ? '' : styleId, wish, item_id: itemId || 0 })) } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  return (
    <Card title={<span className="row tight"><Icon name="sparkles" size={16} />{itemId ? 'What goes with it — ask the AI' : 'Style me'}</span>} sub={itemId ? undefined : 'Say what the day needs; the AI picks from your own clothes.'}>
      <div className="row wrap">
        <input className="grow" value={wish} onChange={e => setWish(e.target.value)} placeholder={itemId ? 'For the office, relaxed…' : 'Dinner with friends, relaxed but smart'} onKeyDown={e => { if (e.key === 'Enter') ask() }} />
        <button type="button" className="btn primary" onClick={ask} disabled={busy}>{busy ? <Spinner /> : <Icon name="sparkles" size={15} />}Ask</button>
      </div>
      {busy && <p className="muted small mt-s">The AI is going through your wardrobe…</p>}
      {res && <div className="ai-outfits">
        {res.outfits.map(o => (
          <div key={o.item_ids.join('-')} className="ai-outfit">
            <OutfitBoard ids={o.item_ids} items={res.items} size="sm" />
            <div className="grow min0">
              <div className="row tight"><b className="grow ellipsis">{o.name}</b><ScoreBadge score={o.rating.score} /></div>
              <p className="small">{o.why}</p>
              <div className="row wrap tight"><button type="button" className="btn sm" onClick={() => onWear(o.item_ids)}>Wear it</button><button type="button" className="btn sm ghost" onClick={() => onSave(o.item_ids)}>Save</button></div>
            </div>
          </div>
        ))}
        {!res.outfits.length && <p className="muted">The AI found nothing complete; add a few more pieces.</p>}
        {res.tip && <p className="tip-line"><Icon name="info" size={14} />{res.tip}</p>}
      </div>}
    </Card>
  )
}
