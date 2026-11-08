// The wardrobe in numbers: how much of it you wear, what each piece cost
// a wear, your colours, and what has not been worn.
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import type { Item } from '../lib/types'
import { money } from '../lib/wardrobe'
import { Bars, HBars, Ring } from '../components/Chart'
import { ItemThumb } from '../components/Pieces'
import { Card, Loading, Segmented, useAsync } from '../components/ui'

interface StatsOut {
  days: number; pieces: number; worn_pieces: number; use_pct: number; days_logged: number; value: number; spent_year: number; currency: string
  most_worn: { item: Item; wears: number }[]; never_worn: { item: Item; wears: number; days_since: number }[]; best_value: Item[]; worst_value: Item[]
  palette: { key: string; n: number; hex: string }[]; categories: { key: string; n: number }[]; weekly: { week: string; n: number }[]; gone: number; donated: number; sold_for: number
}

export default function Stats() {
  const [days, setDays] = useState(90)
  const { data } = useAsync(() => api.get<StatsOut>(`${V1}/stats?days=${days}`), [days])
  if (!data) return <Loading />
  const total = data.palette.reduce((a, p) => a + p.n, 0) || 1
  return (
    <div className="page">
      <div className="page-head row wrap">
        <div className="grow"><h2 className="page-title">Your wardrobe in numbers</h2></div>
        <Segmented size="sm" value={days} onChange={setDays} options={[{ value: 30, label: '30 days' }, { value: 90, label: '3 months' }, { value: 365, label: 'A year' }]} />
      </div>
      <div className="tiles">
        <div className="tile static"><div className="row tight"><Ring pct={data.use_pct} size={56}><b>{data.use_pct}%</b></Ring><div><div className="tile-name">Worn</div><div className="muted small">{data.worn_pieces} of {data.pieces} pieces</div></div></div></div>
        <div className="tile static"><div className="tile-name">Days logged</div><div className="tile-val">{data.days_logged}</div><div className="muted small">of {data.days}</div></div>
        <div className="tile static"><div className="tile-name">Wardrobe value</div><div className="tile-val">{money(data.value, data.currency)}</div><div className="muted small">what you paid</div></div>
        <div className="tile static"><div className="tile-name">Bought this year</div><div className="tile-val">{money(data.spent_year, data.currency)}</div>{data.gone > 0 && <div className="muted small">{data.gone} let go{data.sold_for ? ` · sold for ${money(data.sold_for, data.currency)}` : ''}</div>}</div>
      </div>
      <div className="two-grid">
        <Card title="Days logged a week"><Bars items={data.weekly.map(w => ({ label: w.week.slice(5), value: w.n }))} format={v => String(Math.round(v))} height={140} /></Card>
        <Card title="Your colours" sub="The main colour of each piece.">
          <div className="palette-bar">{data.palette.map(p => <span key={p.key} style={{ flex: p.n / total, background: p.hex }} title={`${p.key}: ${p.n}`} />)}</div>
          <div className="row wrap tight mt-s">{data.palette.slice(0, 10).map(p => <span key={p.key} className="small row tight"><span className="color-dot" style={{ background: p.hex }} />{p.key} {p.n}</span>)}</div>
        </Card>
        <Card title="Most worn">
          <ul className="mini-list">{data.most_worn.filter(x => x.wears > 0).map(x => <li key={x.item.id}><ItemThumb item={x.item} size={36} /><Link to={`/closet/${x.item.id}`} className="grow ellipsis">{x.item.name}</Link><span className="muted small">{x.wears}×</span></li>)}</ul>
        </Card>
        <Card title="Not worn" sub={`In the last ${days} days — wear them, or let them go.`}>
          {data.never_worn.length === 0 ? <p className="muted small">You wore everything. Well done.</p>
            : <ul className="mini-list">{data.never_worn.map(x => <li key={x.item.id}><ItemThumb item={x.item} size={36} /><Link to={`/closet/${x.item.id}`} className="grow ellipsis">{x.item.name}</Link><span className="muted small">{x.days_since < 0 ? 'never' : `${x.days_since} days`}</span></li>)}</ul>}
        </Card>
        {data.best_value.length > 0 && <Card title="Best value" sub="Least cost per wear.">
          <ul className="mini-list">{data.best_value.map(it => <li key={it.id}><ItemThumb item={it} size={36} /><Link to={`/closet/${it.id}`} className="grow ellipsis">{it.name}</Link><span className="muted small">{money(it.per_wear, it.currency)} a wear</span></li>)}</ul>
        </Card>}
        {data.worst_value.length > 0 && <Card title="Most per wear" sub="Expensive for how often they are worn.">
          <ul className="mini-list">{data.worst_value.map(it => <li key={it.id}><ItemThumb item={it} size={36} /><Link to={`/closet/${it.id}`} className="grow ellipsis">{it.name}</Link><span className="muted small">{money(it.per_wear, it.currency)} a wear</span></li>)}</ul>
        </Card>}
        <Card title="What is in it"><HBars items={data.categories.map(c => ({ label: c.key, value: c.n }))} format={v => String(Math.round(v))} /></Card>
      </div>
    </div>
  )
}
