// The laundry basket, the tailor, and what is lent out or stored.
import { useCallback } from 'react'
import { Link } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { announceChange, useOnChange } from '../lib/data'
import { fmtDate } from '../lib/format'
import type { Item, ItemStatus } from '../lib/types'
import { STATUS } from '../lib/wardrobe'
import { ItemThumb } from '../components/Pieces'
import { Icon } from '../components/icons'
import { Card, Empty, Loading, errMsg, useAsync, useToast } from '../components/ui'

export default function Laundry() {
  const { toast } = useToast()
  const { data, reload } = useAsync(() => api.get<{ items: Item[] }>(`${V1}/items`), [])
  useOnChange(useCallback(() => { reload() }, [reload]))
  if (!data) return <Loading />
  const by = (s: ItemStatus) => data.items.filter(i => i.status === s)
  const move = async (ids: number[], status: ItemStatus, from = '') => {
    try { const r = await api.post<{ changed: number }>(`${V1}/items/status`, { ids, status, from }); announceChange(); toast(status === 'ready' ? `${r.changed} back in the closet` : 'Moved') } catch (e) { toast(errMsg(e), 'err') }
  }
  const laundry = by('laundry')
  const groups: [ItemStatus, Item[]][] = [['repair', by('repair')], ['lent', by('lent')], ['stored', by('stored')]]
  return (
    <div className="page">
      <div className="page-head"><h2 className="page-title">Laundry and the tailor</h2><p className="muted">Pieces here are left out of the day's ideas until they are back.</p></div>
      <Card title={<span className="row tight"><Icon name="washer" size={16} />Laundry basket</span>} sub={laundry.length ? `${laundry.length} pieces` : undefined}
        actions={laundry.length > 0 && <button type="button" className="btn primary sm" onClick={() => move([], 'ready', 'laundry')}><Icon name="check" size={14} />All washed</button>}>
        {laundry.length === 0 ? <Empty title="Nothing to wash" icon="check-circle"><p>When you log what you wore, tops and sportswear land here (Settings → General).</p></Empty>
          : <ul className="laundry-list">{laundry.map(it => <Row key={it.id} it={it} onBack={() => move([it.id], 'ready')} />)}</ul>}
      </Card>
      {groups.map(([s, list]) => list.length > 0 && <Card key={s} title={<span className="row tight"><Icon name={STATUS[s].icon} size={16} />{STATUS[s].label}</span>}>
        <ul className="laundry-list">{list.map(it => <Row key={it.id} it={it} onBack={() => move([it.id], 'ready')} />)}</ul>
      </Card>)}
    </div>
  )
}

function Row({ it, onBack }: { it: Item; onBack: () => void }) {
  return (
    <li>
      <Link to={`/closet/${it.id}`} className="row tight grow min0"><ItemThumb item={it} size={48} /><span className="grow min0"><b className="ellipsis block-line">{it.name}</b><span className="muted small">{[it.status_note, it.status_at && `since ${fmtDate(it.status_at)}`, it.care].filter(Boolean).join(' · ')}</span></span></Link>
      <button type="button" className="btn sm" onClick={onBack}><Icon name="hanger" size={13} />Back</button>
    </li>
  )
}
