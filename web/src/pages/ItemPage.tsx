// One piece: its photo, what it is, how often it was worn and what it
// cost each time, what goes with it, and what to do with it.
import { useCallback, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { announceChange, useAppState, useOnChange } from '../lib/data'
import { fmtDate, todayISO } from '../lib/format'
import type { Item, ItemStatus, Outfit } from '../lib/types'
import { FORMAL_WORDS, GONE_WORDS, PATTERN_WORDS, SEASON_WORDS, STATUS, WARMTH_WORDS, catLabel, kindLabel, money, styleName, useCatalog, wornWords } from '../lib/wardrobe'
import { useCrumb } from '../components/Layout'
import { ItemForm, WearForm } from '../components/Forms'
import { ColorDots, ItemCard, ItemThumb, OutfitBoard } from '../components/Pieces'
import { Icon } from '../components/icons'
import { Card, Confirm, Field, Loading, Modal, MenuButton, errMsg, useAsync, useToast } from '../components/ui'
import { Select } from '../components/Select'
import { StyleMe } from './Today'

interface ItemOut { item: Item; worn: string[]; outfits: Outfit[]; similar: Item[]; goes_with: Item[]; nudge?: string }

export default function ItemPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const cat = useCatalog()
  const { toast } = useToast()
  const appState = useAppState()
  const tasks = appState('tasks', 'tasks:write')
  const { data, reload, error } = useAsync(() => api.get<ItemOut>(`${V1}/items/${id}`), [id])
  useOnChange(useCallback(() => { reload() }, [reload]))
  const [edit, setEdit] = useState(false)
  const [wear, setWear] = useState(false)
  const [gone, setGone] = useState(false)
  const [del, setDel] = useState(false)
  const [task, setTask] = useState<string | null>(null)
  const [lent, setLent] = useState(false)
  const [who, setWho] = useState('')
  useCrumb(data?.item.name)
  if (error) return <div className="page"><p className="muted">{errMsg(error)}</p></div>
  if (!data) return <Loading />
  const it = data.item
  const setStatus = async (s: ItemStatus, note = '') => {
    try { await api.post(`${V1}/items/status`, { ids: [it.id], status: s, note }); announceChange(); toast(STATUS[s].label) } catch (e) { toast(errMsg(e), 'err') }
  }
  const fav = async () => { await api.put(`${V1}/items/${it.id}`, { favourite: !it.favourite }); announceChange() }
  const items: Record<string, Item> = { [it.id]: it }
  for (const x of [...data.goes_with, ...data.similar]) items[x.id] = x
  return (
    <div className="page det-page">
      <div className="item-top">
        <div className="item-photo">{it.photo_url ? <img src={it.photo_url} alt={it.name} /> : <ItemThumb item={it} size={260} />}</div>
        <div className="item-info stack">
          <div className="det-kicker">{catLabel(cat, it.category)} · {kindLabel(cat, it.kind)}</div>
          <h2 className="det-title">{it.name}</h2>
          <div className="row wrap tight"><ColorDots colors={it.colors} size={18} /><span className="muted">{it.colors.map(c => c.name).join(', ')}{it.pattern !== 'plain' ? ` · ${PATTERN_WORDS[it.pattern]}` : ''}</span></div>
          <div className="row wrap tight">
            <span className={`pill ${STATUS[it.status].cls}`}><Icon name={STATUS[it.status].icon} size={13} />{STATUS[it.status].label}{it.status_note ? ` — ${it.status_note}` : ''}</span>
            {it.gone && <span className="pill st-done"><Icon name="archive" size={13} />{GONE_WORDS[it.gone]} {it.gone_on && fmtDate(it.gone_on)}</span>}
          </div>
          <div className="row wrap">
            {!it.gone && <button type="button" className="btn primary" onClick={() => setWear(true)}><Icon name="check" size={15} />Wore it today</button>}
            {it.status === 'ready' ? <button type="button" className="btn" onClick={() => setStatus('laundry')}><Icon name="washer" size={15} />To the laundry</button>
              : <button type="button" className="btn" onClick={() => setStatus('ready')}><Icon name="hanger" size={15} />Back in the closet</button>}
            <button type="button" className={`btn ${it.favourite ? 'on' : ''}`} onClick={fav} aria-pressed={it.favourite}><Icon name="heart" size={15} />{it.favourite ? 'Favourite' : 'Love it'}</button>
            <button type="button" className="btn" onClick={() => setEdit(true)}><Icon name="edit" size={15} />Change</button>
            <MenuButton label="More" items={[
              { label: 'At the tailor / to repair', icon: 'scissors', onSelect: () => setStatus('repair', 'to repair') },
              { label: 'Lent to someone', icon: 'users', onSelect: () => setLent(true) },
              { label: 'Stored away (out of season)', icon: 'archive', onSelect: () => setStatus('stored') },
              'sep',
              { label: 'Take to the tailor (Pocket Tasks)', icon: 'scissors', disabled: !tasks.ok, onSelect: () => setTask('tailor') },
              { label: 'Dry clean (Pocket Tasks)', icon: 'droplet', disabled: !tasks.ok, onSelect: () => setTask('dry_clean') },
              { label: 'Return it (Pocket Tasks)', icon: 'package', disabled: !tasks.ok, onSelect: () => setTask('return') },
              'sep',
              { label: it.gone ? 'I have it again' : 'Gave it away, sold it…', icon: 'archive', onSelect: () => it.gone ? api.post(`${V1}/items/${it.id}/gone`, { reason: '' }).then(() => announceChange()) : setGone(true) },
              { label: 'Delete', icon: 'trash', danger: true, onSelect: () => setDel(true) },
            ]}><Icon name="more" size={18} /></MenuButton>
          </div>
          {!tasks.ok && <p className="muted tiny">Reminders for the tailor and returns need Pocket Tasks{tasks.app?.fix ? ` — ${tasks.app.fix}` : ''}.</p>}
          {data.nudge && <div className="callout info"><Icon name="info" size={15} /><div>{data.nudge}</div></div>}
          <dl className="facts">
            <div><dt>Worn</dt><dd>{wornWords(it)}</dd></div>
            {it.price != null && <div><dt>Cost</dt><dd>{money(it.price, it.currency)}{it.per_wear != null && <span className="muted"> · {money(it.per_wear, it.currency)} a wear</span>}</dd></div>}
            <div><dt>Warmth</dt><dd>{WARMTH_WORDS[it.warmth]}</dd></div>
            <div><dt>How smart</dt><dd>{FORMAL_WORDS[it.formality]}</dd></div>
            <div><dt>Seasons</dt><dd>{it.seasons.length ? it.seasons.map(s => SEASON_WORDS[s]).join(', ') : 'All year'}</dd></div>
            {it.styles.length > 0 && <div><dt>Styles</dt><dd>{it.styles.map(s => styleName(cat, s)).join(', ')}</dd></div>}
            {(it.brand || it.size) && <div><dt>Brand, size</dt><dd>{[it.brand, it.size].filter(Boolean).join(' · ')}</dd></div>}
            {it.material && <div><dt>Material</dt><dd>{it.material}{it.fit ? ` · ${it.fit}` : ''}</dd></div>}
            {(it.bought_on || it.shop) && <div><dt>Bought</dt><dd>{[it.bought_on && fmtDate(it.bought_on), it.shop].filter(Boolean).join(' · ')}</dd></div>}
            {it.care && <div><dt>Care</dt><dd>{it.care}</dd></div>}
            {it.rain && <div><dt>Rain</dt><dd>Keeps it out</dd></div>}
            {it.notes && <div><dt>Notes</dt><dd>{it.notes}</dd></div>}
          </dl>
        </div>
      </div>

      {data.goes_with.length > 0 && <Card title="Goes best with" sub="From your own closet, by colour, how smart and the season." actions={<Link className="btn sm" to={`/outfits/ideas?include=${it.id}`}><Icon name="shuffle" size={14} />Outfits with it</Link>}>
        <div className="item-grid small">{data.goes_with.map(x => <ItemCard key={x.id} item={x} to={`/closet/${x.id}`} />)}</div>
      </Card>}
      <StyleMe date={todayISO()} occasion="everyday" styleId="" itemId={it.id} onWear={() => setWear(true)} onSave={ids => api.post(`${V1}/outfits`, { item_ids: ids, made_by: 'ai' }).then(() => { announceChange(); toast('Saved as an outfit') })} />
      {data.outfits.length > 0 && <Card title="In your outfits">
        <div className="outfit-grid">{data.outfits.map(o => <Link key={o.id} to={`/outfits/${o.id}`} className="outfit-card"><OutfitBoard ids={o.item_ids} items={items} size="sm" /><span className="ic-name ellipsis">{o.name}</span></Link>)}</div>
      </Card>}
      {data.similar.length > 0 && <Card title="Much like it" sub="Same kind of piece, close colour: worth knowing before buying another.">
        <div className="item-grid small">{data.similar.map(x => <ItemCard key={x.id} item={x} to={`/closet/${x.id}`} />)}</div>
      </Card>}
      {data.worn.length > 0 && <Card title="Days worn"><div className="chips">{data.worn.map(d => <Link key={d} className="chip" to={`/plan?date=${d}`}>{fmtDate(d)}</Link>)}</div></Card>}

      {edit && <ItemForm item={it} onClose={() => setEdit(false)} />}
      {wear && <WearForm ids={[it.id]} items={items} onClose={() => setWear(false)} />}
      {gone && <GoneForm item={it} onClose={() => setGone(false)} />}
      {lent && <Modal title="Lent to someone" onClose={() => setLent(false)} size="sm" footer={<><button type="button" className="btn" onClick={() => setLent(false)}>Cancel</button><button type="button" className="btn primary" onClick={() => { setStatus('lent', who.trim()); setLent(false) }}>Save</button></>}>
        <Field label="Who has it?"><input autoFocus value={who} onChange={e => setWho(e.target.value)} placeholder="Tarik" /></Field>
      </Modal>}
      {task && <TaskForm item={it} what={task} onClose={() => setTask(null)} />}
      {del && <Confirm title="Delete this piece?" message="It goes for good, with its photo and its place in your outfits. Its days worn stay without it. To keep its history, use “Gave it away, sold it…” instead." confirmLabel="Delete" onCancel={() => setDel(false)}
        onConfirm={async () => { await api.del(`${V1}/items/${it.id}`); announceChange(); navigate('/closet') }} />}
    </div>
  )
}

function GoneForm({ item, onClose }: { item: Item; onClose: () => void }) {
  const { toast } = useToast()
  const [reason, setReason] = useState('donated')
  const [sold, setSold] = useState('')
  const save = async () => {
    try { await api.post(`${V1}/items/${item.id}/gone`, { reason, date: todayISO(), sold_for: reason === 'sold' && sold ? Math.round(parseFloat(sold.replace(',', '.')) * 100) : null }); announceChange(); toast('Done — its history stays'); onClose() } catch (e) { toast(errMsg(e), 'err') }
  }
  return (
    <Modal title="No longer yours" onClose={onClose} size="sm" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" onClick={save}>Save</button></>}>
      <div className="stack">
        <Field label="What happened"><Select ariaLabel="Reason" value={reason} onChange={setReason} options={Object.entries(GONE_WORDS).map(([value, label]) => ({ value, label }))} /></Field>
        {reason === 'sold' && <Field label="Sold for"><input inputMode="decimal" value={sold} onChange={e => setSold(e.target.value)} placeholder="0.00" /></Field>}
      </div>
    </Modal>
  )
}

function TaskForm({ item, what, onClose }: { item: Item; what: string; onClose: () => void }) {
  const { toast } = useToast()
  const [due, setDue] = useState('')
  const [note, setNote] = useState('')
  const title = { tailor: 'Take to the tailor', dry_clean: 'Dry clean', return: 'Return it' }[what] || 'Reminder'
  const save = async () => {
    try { const r = await api.post<{ text: string }>(`${V1}/items/${item.id}/task`, { what, due, note }); announceChange(); toast(r.text); onClose() } catch (e) { toast(errMsg(e), 'err') }
  }
  return (
    <Modal title={title} onClose={onClose} size="sm" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" onClick={save}>Add to Pocket Tasks</button></>}>
      <div className="stack">
        <Field label="By"><input type="date" value={due} onChange={e => setDue(e.target.value)} min={todayISO()} /></Field>
        <Field label="Note"><input value={note} onChange={e => setNote(e.target.value)} placeholder={what === 'tailor' ? 'Shorten by 2 cm' : ''} /></Field>
      </div>
    </Modal>
  )
}
