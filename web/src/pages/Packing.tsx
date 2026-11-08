// Packing for trips: from Pocket Travel or typed in; a small set of
// pieces that mixes into many outfits for the days and the weather there,
// ticked off as it goes in the suitcase.
import { useCallback, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, useAppState, useOnChange } from '../lib/data'
import { daysBetween, fmtDate, fmtRange } from '../lib/format'
import type { Item, Pack, Score } from '../lib/types'
import { PURPOSES, tempWords, weatherIcon } from '../lib/wardrobe'
import { openCreate, useCrumb } from '../components/Layout'
import { PackForm, PiecePicker } from '../components/Forms'
import { ItemThumb, OutfitBoard } from '../components/Pieces'
import { Icon } from '../components/icons'
import { Card, Confirm, Empty, Loading, Modal, Spinner, errMsg, useAsync, useToast } from '../components/ui'

export default function Packing() {
  const { data, reload } = useAsync(() => api.get<{ packs: Pack[] }>(`${V1}/packs`), [])
  useOnChange(useCallback(() => { reload() }, [reload]))
  if (!data) return <Loading />
  return (
    <div className="page">
      <div className="page-head row wrap">
        <div className="grow"><h2 className="page-title">Packing</h2><p className="muted">A few pieces that go with each other, for the days and the weather of the trip.</p></div>
        <button type="button" className="btn primary" onClick={() => openCreate('pack')}><Icon name="suitcase" size={15} />Pack for a trip</button>
      </div>
      {data.packs.length === 0 ? <Empty title="No trips yet" icon="suitcase"><p>Start from a trip in Pocket Travel, or type where you are going.</p></Empty>
        : <Card flush><ul className="list">{data.packs.map(p => {
          const packed = p.items.filter(i => i.packed).length + p.extras.filter(e => e.packed).length
          const total = p.items.length + p.extras.length
          return <li key={p.id}><Link to={`/packing/${p.id}`} className="list-row">
            <span className="list-ico"><Icon name={PURPOSES.find(x => x.value === p.purpose)?.icon || 'suitcase'} size={16} /></span>
            <div className="grow min0"><b className="ellipsis">{p.name}</b><div className="muted small">{[p.start_date && fmtRange(p.start_date, p.end_date || p.start_date), p.destination !== p.name && p.destination, p.weather && tempWords(p.weather)].filter(Boolean).join(' · ')}</div></div>
            <span className="muted small">{total ? `${packed}/${total} packed` : 'empty'}</span>
          </Link></li>
        })}</ul></Card>}
    </div>
  )
}

export function PackPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { settings } = useApp()
  const { toast } = useToast()
  const appState = useAppState()
  const notes = appState('notes', 'notes:write')
  const { data, reload } = useAsync(() => api.get<{ pack: Pack; items: Record<string, Item>; outfits: number; examples: Score[] }>(`${V1}/packs/${id}`), [id])
  useOnChange(useCallback(() => { reload() }, [reload]))
  const [busy, setBusy] = useState('')
  const [edit, setEdit] = useState(false)
  const [pick, setPick] = useState(false)
  const [del, setDel] = useState(false)
  const [extra, setExtra] = useState('')
  useCrumb(data?.pack.name)
  if (!data) return <Loading />
  const p = data.pack
  const put = async (patch: Partial<Pack>) => { try { await api.put(`${V1}/packs/${p.id}`, patch); reload() } catch (e) { toast(errMsg(e), 'err') } }
  const fill = async () => { setBusy('fill'); try { const r = await api.post<{ text: string; notes: string[] }>(`${V1}/packs/${p.id}/fill`); toast(r.text); r.notes.forEach(n => toast(n)); reload() } catch (e) { toast(errMsg(e), 'err') } finally { setBusy('') } }
  const ai = async () => {
    setBusy('ai')
    try {
      const r = await api.post<{ item_ids: number[]; extras: string[]; tip: string }>(`${V1}/packs/${p.id}/ai`)
      const have = new Set(p.items.map(i => i.item_id))
      const haveX = new Set(p.extras.map(e => e.text.toLowerCase()))
      await put({ items: [...p.items, ...r.item_ids.filter(i => !have.has(i)).map(i => ({ item_id: i, qty: 1, packed: false }))], extras: [...p.extras, ...r.extras.filter(e => !haveX.has(e.toLowerCase())).map(text => ({ text, packed: false }))] })
      if (r.tip) toast(r.tip)
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy('') }
  }
  const toNote = async () => { try { const r = await api.post<{ text: string }>(`${V1}/packs/${p.id}/note`); toast(r.text) } catch (e) { toast(errMsg(e), 'err') } }
  const days = p.start_date && p.end_date ? daysBetween(p.start_date, p.end_date) + 1 : 0
  const packed = p.items.filter(i => i.packed).length + p.extras.filter(e => e.packed).length
  const total = p.items.length + p.extras.length
  return (
    <div className="page det-page">
      <div className="det-head">
        <div className="grow min0">
          <div className="det-kicker">{PURPOSES.find(x => x.value === p.purpose)?.label}{days ? ` · ${days} days` : ''}</div>
          <h2 className="det-title">{p.name}</h2>
          <p className="muted">{[p.destination !== p.name && p.destination, p.start_date && fmtRange(p.start_date, p.end_date || p.start_date)].filter(Boolean).join(' · ')}{p.trip_url && <> · <a href={p.trip_url} target="_blank" rel="noreferrer">the trip in Pocket Travel</a></>}</p>
          {p.weather && <span className="weather-chip"><Icon name={weatherIcon(p.weather.code)} size={15} />{tempWords(p.weather)} · {p.weather.condition?.toLowerCase()}</span>}
        </div>
        <div className="det-tools row wrap">
          <button type="button" className="btn primary" onClick={fill} disabled={!!busy}>{busy === 'fill' ? <Spinner /> : <Icon name="layers" size={15} />}Pick for me</button>
          {settings?.ai_help && <button type="button" className="btn" onClick={ai} disabled={!!busy}>{busy === 'ai' ? <Spinner /> : <Icon name="sparkles" size={15} />}Ask the AI</button>}
          <button type="button" className="btn" onClick={() => setPick(true)}><Icon name="plus" size={15} />Add pieces</button>
          <button type="button" className="btn" onClick={toNote} disabled={!notes.ok} title={notes.ok ? 'Save the list in Pocket Notes' : `Pocket Notes: ${notes.app?.detail || 'not connected'}`}><Icon name="note" size={15} />To Notes</button>
          <button type="button" className="btn ghost" onClick={() => setEdit(true)}><Icon name="edit" size={15} /></button>
          <button type="button" className="btn ghost danger-text" onClick={() => setDel(true)} aria-label="Delete"><Icon name="trash" size={15} /></button>
        </div>
      </div>
      {total > 0 && <div className="packed-bar"><span style={{ width: `${Math.round(packed / total * 100)}%` }} /><b>{packed} of {total} in the suitcase</b></div>}
      <div className="two-grid">
        <Card title="Clothes" sub={p.items.length ? `${p.items.length} pieces · about ${data.outfits} outfits` : 'Nothing yet: “Pick for me” chooses a small set that mixes well.'}>
          <ul className="pack-list">{p.items.map(pi => {
            const it = data.items[String(pi.item_id)]
            if (!it) return null
            return <li key={pi.item_id} className={pi.packed ? 'done' : ''}>
              <label className="row tight grow"><input type="checkbox" checked={pi.packed} onChange={e => put({ items: p.items.map(x => x.item_id === pi.item_id ? { ...x, packed: e.target.checked } : x) })} /><ItemThumb item={it} size={40} /><span className="grow ellipsis">{it.name}</span></label>
              {it.status !== 'ready' && <span className="pill st-due small">{it.status}</span>}
              <button type="button" className="btn ghost icon-only sm" aria-label="Take out" onClick={() => put({ items: p.items.filter(x => x.item_id !== pi.item_id) })}><Icon name="x" size={14} /></button>
            </li>
          })}</ul>
        </Card>
        <Card title="Also bring">
          <ul className="pack-list">{p.extras.map((e, i) => <li key={i} className={e.packed ? 'done' : ''}>
            <label className="row tight grow"><input type="checkbox" checked={e.packed} onChange={ev => put({ extras: p.extras.map((x, j) => j === i ? { ...x, packed: ev.target.checked } : x) })} /><span className="grow">{e.text}</span></label>
            <button type="button" className="btn ghost icon-only sm" aria-label="Remove" onClick={() => put({ extras: p.extras.filter((_, j) => j !== i) })}><Icon name="x" size={14} /></button>
          </li>)}</ul>
          <div className="row tight mt-s"><input className="grow" value={extra} onChange={e => setExtra(e.target.value)} placeholder="Charger, passport…" onKeyDown={e => { if (e.key === 'Enter' && extra.trim()) { put({ extras: [...p.extras, { text: extra.trim(), packed: false }] }); setExtra('') } }} /><button type="button" className="btn" disabled={!extra.trim()} onClick={() => { put({ extras: [...p.extras, { text: extra.trim(), packed: false }] }); setExtra('') }}>Add</button></div>
        </Card>
      </div>
      {data.examples.length > 0 && <Card title="Outfits from the suitcase"><div className="ib-examples">{data.examples.map(ex => <OutfitBoard key={ex.items.join('-')} ids={ex.items} items={data.items} size="sm" />)}</div></Card>}
      {pick && <PickModal pack={p} onClose={() => setPick(false)} onSave={ids => { put({ items: [...p.items, ...ids.filter(i => !p.items.some(x => x.item_id === i)).map(i => ({ item_id: i, qty: 1, packed: false }))] }); setPick(false) }} />}
      {edit && <PackForm pack={p} onClose={() => setEdit(false)} onSaved={() => reload()} />}
      {del && <Confirm title="Delete this packing list?" message="The pieces stay in your closet." confirmLabel="Delete" onCancel={() => setDel(false)} onConfirm={async () => { await api.del(`${V1}/packs/${p.id}`); announceChange(); navigate('/packing') }} />}
      <p className="muted tiny">{p.start_date && `Made ${fmtDate(p.created_at.slice(0, 10))}`}</p>
    </div>
  )
}

function PickModal({ pack, onClose, onSave }: { pack: Pack; onClose: () => void; onSave: (ids: number[]) => void }) {
  const { data } = useAsync(() => api.get<{ items: Item[] }>(`${V1}/items`), [])
  const [ids, setIds] = useState<number[]>(pack.items.map(i => i.item_id))
  return (
    <Modal title="Add pieces" onClose={onClose} size="lg" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" onClick={() => onSave(ids)}>Add</button></>}>
      {data ? <PiecePicker items={data.items} chosen={ids} onChange={setIds} /> : <Loading />}
    </Modal>
  )
}
