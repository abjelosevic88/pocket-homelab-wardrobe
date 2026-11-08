// The closet: every piece as a picture, by kind, with a search and the
// states (in the laundry, at the tailor, lent).
import { useCallback, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useOnChange } from '../lib/data'
import type { Item, ItemStatus } from '../lib/types'
import { GONE_WORDS, STATUS, money, useCatalog, wornWords } from '../lib/wardrobe'
import { openCreate } from '../components/Layout'
import { ItemCard } from '../components/Pieces'
import { Icon } from '../components/icons'
import { Select } from '../components/Select'
import { Empty, Loading, useAsync, useDebounce } from '../components/ui'

interface ListOut { items: Item[]; counts: Record<string, number>; states: Record<string, number>; total: number }

export default function Closet() {
  const cat = useCatalog()
  const [params, setParams] = useSearchParams()
  const category = params.get('c') || ''
  const status = params.get('s') || ''
  const gone = params.get('gone') === '1'
  const [q, setQ] = useState('')
  const dq = useDebounce(q.trim(), 250)
  const [sort, setSort] = useState('kind')
  const set = (k: string, v: string) => { const p = new URLSearchParams(params); if (v) p.set(k, v); else p.delete(k); setParams(p, { replace: true }) }
  const { data, reload } = useAsync(() => {
    const qs = new URLSearchParams()
    if (category) qs.set('category', category)
    if (status) qs.set('status', status)
    if (dq) qs.set('q', dq)
    if (gone) qs.set('gone', '1')
    return api.get<ListOut>(`${V1}/items?${qs}`)
  }, [category, status, dq, gone])
  useOnChange(useCallback(() => { reload() }, [reload]))
  if (!data) return <Loading />
  const list = [...data.items]
  if (sort === 'worn') list.sort((a, b) => b.wears - a.wears)
  if (sort === 'least') list.sort((a, b) => a.wears - b.wears || (a.last_worn || '').localeCompare(b.last_worn || ''))
  if (sort === 'new') list.sort((a, b) => (b.bought_on || b.created_at).localeCompare(a.bought_on || a.created_at))
  if (sort === 'colour') list.sort((a, b) => (a.colors[0]?.name || '').localeCompare(b.colors[0]?.name || ''))
  return (
    <div className="page">
      <div className="page-head row wrap">
        <div className="grow"><h2 className="page-title">{gone ? 'Given away and sold' : 'Closet'}</h2><p className="muted">{gone ? 'Pieces you no longer have; their history stays.' : `${data.total} pieces`}</p></div>
        <button type="button" className="btn" onClick={() => openCreate('many')}><Icon name="image" size={15} />From one photo</button>
        <button type="button" className="btn primary" onClick={() => openCreate('item')}><Icon name="camera" size={15} />Add a piece</button>
      </div>
      <div className="filters row wrap">
        <div className="search-in grow"><Icon name="search" size={15} /><input value={q} onChange={e => setQ(e.target.value)} placeholder="Find: navy, wool, Zara…" aria-label="Search the closet" /></div>
        <Select ariaLabel="Where it is" value={status} onChange={v => set('s', v)} options={[{ value: '', label: 'Anywhere' }, ...(Object.keys(STATUS) as ItemStatus[]).map(s => ({ value: s, label: `${STATUS[s].label}${data.states[s] ? ` (${data.states[s]})` : ''}` }))]} />
        <Select ariaLabel="Order" value={sort} onChange={setSort} options={[{ value: 'kind', label: 'By kind' }, { value: 'worn', label: 'Most worn' }, { value: 'least', label: 'Least worn' }, { value: 'new', label: 'Newest' }, { value: 'colour', label: 'By colour' }]} />
      </div>
      <div className="chips scroll-x">
        <button type="button" className={`chip ${!category ? 'on' : ''}`} onClick={() => set('c', '')}>All <span className="count">{data.total}</span></button>
        {(cat?.categories || []).filter(c => data.counts[c.id]).map(c => <button key={c.id} type="button" className={`chip ${category === c.id ? 'on' : ''}`} onClick={() => set('c', c.id)}>{c.plural} <span className="count">{data.counts[c.id]}</span></button>)}
        <button type="button" className={`chip ${gone ? 'on' : ''}`} onClick={() => set('gone', gone ? '' : '1')}><Icon name="archive" size={13} />No longer mine</button>
      </div>
      {list.length === 0 ? (data.total === 0 && !gone
        ? <Empty title="Your closet is empty" icon="hanger"><p>Photograph a piece (on a hanger or laid flat on a bed works best). With AI switched on, the photo fills in the rest — colour, kind, season, how smart and how warm.</p><div className="row wrap center"><button type="button" className="btn primary" onClick={() => openCreate('item')}><Icon name="camera" size={15} />Add a piece</button><button type="button" className="btn" onClick={() => openCreate('many')}><Icon name="image" size={15} />Several from one photo</button></div></Empty>
        : <Empty title="Nothing here" icon="search" />)
        : <div className="item-grid">
          {list.map(it => <ItemCard key={it.id} item={it} to={`/closet/${it.id}`}
            badge={it.status !== 'ready' && !gone ? <span className={`ic-state ${STATUS[it.status].cls}`} title={STATUS[it.status].label}><Icon name={STATUS[it.status].icon} size={12} /></span> : undefined}
            sub={gone ? `${GONE_WORDS[it.gone] || it.gone}${it.gone_on ? ' · ' + it.gone_on : ''}` : sort === 'worn' || sort === 'least' ? wornWords(it) : it.per_wear != null && sort === 'kind' ? `${money(it.per_wear, it.currency)} a wear` : it.brand || wornWords(it)} />)}
        </div>}
    </div>
  )
}
