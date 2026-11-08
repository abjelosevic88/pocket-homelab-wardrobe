// Plan the days ahead (each with its forecast and what is on), and look
// back at what you wore.
import { useCallback, useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { announceChange, useOnChange } from '../lib/data'
import { addDays, fmtDate, todayISO, weekday } from '../lib/format'
import type { DayPlan, Item, Wear } from '../lib/types'
import { occasionLabel, tempWords, useCatalog, weatherIcon } from '../lib/wardrobe'
import { WearForm } from '../components/Forms'
import { OutfitBoard } from '../components/Pieces'
import { Icon } from '../components/icons'
import { Card, Loading, Spinner, Tabs, errMsg, useAsync, useToast } from '../components/ui'

export default function Plan() {
  const [params, setParams] = useSearchParams()
  const tab = (params.get('tab') || (params.get('date') && params.get('date')! < todayISO() ? 'history' : 'ahead')) as 'ahead' | 'history'
  return (
    <div className="page">
      <div className="page-head"><h2 className="page-title">Plan and history</h2></div>
      <Tabs tabs={[{ id: 'ahead', label: 'The week ahead', icon: 'calendar-plus' }, { id: 'history', label: 'What you wore', icon: 'calendar' }]} value={tab} onChange={t => setParams({ tab: t }, { replace: true })} />
      <div className="mt">{tab === 'ahead' ? <Ahead /> : <History focus={params.get('date') || ''} />}</div>
    </div>
  )
}

function Ahead() {
  const days = useMemo(() => Array.from({ length: 7 }, (_, i) => addDays(todayISO(), i)), [])
  return <div className="stack loose">{days.map(d => <DayRow key={d} date={d} />)}</div>
}

/** One day ahead: the forecast, the events, the plan or ideas for it. */
function DayRow({ date }: { date: string }) {
  const cat = useCatalog()
  const { toast } = useToast()
  const [shuffle, setShuffle] = useState(0)
  const { data, reload } = useAsync(() => api.get<{ day: DayPlan }>(`${V1}/today?date=${date}&shuffle=${shuffle}`), [date, shuffle])
  useOnChange(useCallback(() => { reload() }, [reload]))
  const [pick, setPick] = useState<number[] | null>(null)
  if (!data) return <Card title={fmtDate(date)}><Spinner /></Card>
  const d = data.day
  const label = date === todayISO() ? 'Today' : date === addDays(todayISO(), 1) ? 'Tomorrow' : weekday(date, true)
  const remove = async (w: Wear) => { try { await api.del(`${V1}/wears/${w.id}`); announceChange() } catch (e) { toast(errMsg(e), 'err') } }
  return (
    <Card title={<span>{label} <span className="muted small">{fmtDate(date, false)}</span></span>}
      sub={[d.events.map(e => e.title).join(', '), d.training ? `Training: ${d.training.name}` : '', d.outing ? d.outing.title : ''].filter(Boolean).join(' · ') || undefined}
      actions={d.weather && <span className="weather-chip"><Icon name={weatherIcon(d.weather.code)} size={15} />{tempWords(d.weather)}</span>}>
      {d.plan ? <div className="plan-row">
        <OutfitBoard ids={d.plan.item_ids} items={d.items} size="sm" />
        <div className="grow"><b>Planned</b>{d.plan.event && <span className="muted small"> · for {d.plan.event}</span>}<div className="row wrap tight mt-s"><button type="button" className="btn sm ghost" onClick={() => remove(d.plan!)}>Remove the plan</button></div></div>
      </div> : <div className="plan-ideas">
        <span className="muted small">Ideas for {occasionLabel(cat, d.occasion).toLowerCase()}{d.because ? ` (${d.because})` : ''} — tap one to plan it:</span>
        <div className="alt-strip">
          {d.suggest.suggestions.slice(0, 4).map(s => <button key={s.items.join('-')} type="button" className="alt" onClick={() => setPick(s.items)} title="Plan this"><OutfitBoard ids={s.items} items={d.items} size="sm" /></button>)}
          <button type="button" className="alt more" onClick={() => setShuffle(x => x + 1)} title="Other ideas"><Icon name="shuffle" size={18} /></button>
        </div>
      </div>}
      {pick && <WearForm ids={pick} items={d.items} date={date} planned occasion={d.occasion} event={d.because || ''} onClose={() => setPick(null)} />}
    </Card>
  )
}

function monthStart(iso: string) { return iso.slice(0, 8) + '01' }

function History({ focus }: { focus: string }) {
  const [month, setMonth] = useState(monthStart(focus || todayISO()))
  const end = addDays(monthStart(addDays(month, 32)), -1)
  const { data, reload } = useAsync(() => api.get<{ wears: Wear[]; items: Record<string, Item> }>(`${V1}/wears?from=${month}&to=${end}`), [month])
  useOnChange(useCallback(() => { reload() }, [reload]))
  const [open, setOpen] = useState<string>(focus)
  if (!data) return <Loading />
  const byDay: Record<string, Wear[]> = {}
  for (const w of data.wears) (byDay[w.date] = byDay[w.date] || []).push(w)
  const first = new Date(month + 'T12:00:00')
  const pad = (first.getDay() + 6) % 7
  const days: (string | null)[] = [...Array(pad).fill(null)]
  for (let d = month; d <= end; d = addDays(d, 1)) days.push(d)
  const sel = open && byDay[open]
  const logged = Object.values(byDay).filter(l => l.some(w => !w.planned)).length
  return (
    <div className="stack loose">
      <div className="row">
        <button type="button" className="btn ghost icon-only" aria-label="Month before" onClick={() => setMonth(monthStart(addDays(month, -1)))}><Icon name="chevron-left" size={18} /></button>
        <h3 className="grow center m0">{first.toLocaleDateString(undefined, { month: 'long', year: 'numeric' })}</h3>
        <button type="button" className="btn ghost icon-only" aria-label="Next month" onClick={() => setMonth(monthStart(addDays(end, 1)))} disabled={addDays(end, 1) > todayISO()}><Icon name="chevron-right" size={18} /></button>
      </div>
      <p className="muted small center">{logged} days logged this month</p>
      <div className="month-grid">
        {['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'].map(w => <span key={w} className="mg-h">{w}</span>)}
        {days.map((d, i) => {
          if (!d) return <span key={'p' + i} />
          const w = (byDay[d] || []).find(x => !x.planned) || (byDay[d] || [])[0]
          const main = w ? w.item_ids.map(id => data.items[String(id)]).filter(Boolean).find(it => ['top', 'knit', 'dress', 'outerwear'].includes(it.category)) : undefined
          return (
            <button key={d} type="button" className={`mg-day ${open === d ? 'on' : ''} ${d === todayISO() ? 'today' : ''} ${w?.planned ? 'planned' : ''}`} onClick={() => setOpen(d)} disabled={!w}>
              <span className="mg-n">{Number(d.slice(8))}</span>
              {main && (main.thumb_url ? <img src={main.thumb_url} alt="" /> : <span className="mg-dot" style={{ background: main.colors[0]?.hex }} />)}
            </button>
          )
        })}
      </div>
      {sel && <Card title={fmtDate(open)}>{sel.map(w => (
        <div key={w.id} className="plan-row">
          <OutfitBoard ids={w.item_ids} items={data.items} size="sm" />
          <div className="grow small">
            {w.planned && <b>Planned · </b>}{w.weather?.t_max != null && <span className="muted">{tempWords(w.weather)} {w.weather.condition?.toLowerCase()} · </span>}
            {w.event && <span>{w.event} · </span>}{w.notes}
            {w.photo_url && <img src={w.photo_url} alt="" className="look-mini" />}
            <div className="mt-s">{w.item_ids.map(id => data.items[String(id)]).filter(Boolean).map(it => <Link key={it.id} className="chip sm" to={`/closet/${it.id}`}>{it.name}</Link>)}</div>
          </div>
        </div>))}</Card>}
    </div>
  )
}
