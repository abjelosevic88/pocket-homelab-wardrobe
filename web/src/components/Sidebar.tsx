// Left column: add a piece (and the other ways to start), the sections,
// then what is coming up.
import { useState } from 'react'
import { Link, NavLink, useLocation, useNavigate } from 'react-router-dom'
import { useApp } from '../lib/app-context'
import { useData } from '../lib/data'
import { fmtDate } from '../lib/format'
import { Icon } from './icons'
import { createItems, type CreateKind } from './Layout'
import { type Anchor, Menu, rectOf } from './ui'

export function Sidebar({ onCreate, onNavigate }: { onCreate: (k: CreateKind) => void; onNavigate: () => void }) {
  const { status, settings } = useApp()
  const { summary } = useData()
  const navigate = useNavigate()
  const loc = useLocation()
  const [menu, setMenu] = useState<Anchor | null>(null)
  const title = settings?.title || status?.title || 'Pocket Wardrobe'
  const path = loc.pathname
  const nav: { to: string; label: string; icon: string; active: boolean; count?: number; hint?: string }[] = [
    { to: '/', label: 'Today', icon: 'today', active: path === '/' },
    { to: '/closet', label: 'Closet', icon: 'hanger', active: path.startsWith('/closet'), count: summary?.items || undefined, hint: 'Pieces in your wardrobe' },
    { to: '/outfits', label: 'Outfits', icon: 'layers', active: path.startsWith('/outfits'), count: summary?.outfits || undefined },
    { to: '/plan', label: 'Plan and history', icon: 'calendar', active: path.startsWith('/plan') },
    { to: '/style', label: 'Your style', icon: 'palette', active: path.startsWith('/style') },
    { to: '/shop', label: 'Worth buying', icon: 'cart', active: path.startsWith('/shop'), count: summary?.ideas || undefined, hint: 'Ideas you kept' },
    { to: '/packing', label: 'Packing', icon: 'suitcase', active: path.startsWith('/packing') },
    { to: '/laundry', label: 'Laundry', icon: 'washer', active: path === '/laundry', count: summary?.laundry || undefined, hint: 'Pieces waiting to be washed' },
    { to: '/stats', label: 'In numbers', icon: 'chart', active: path === '/stats' },
  ]
  const trip = summary?.next_trip
  return (
    <aside className="sidebar" aria-label="Pocket Wardrobe">
      <div className="sb-brand">
        <img src="/favicon.svg" alt="" className="sb-logo" />
        <span className="sb-title">{title !== 'Pocket Wardrobe' ? title : <>Pocket <b>Wardrobe</b></>}</span>
        {status?.version && <span className="sb-version" title={`Version ${status.version}`}>{/^v?\d+\.\d+/.test(status.version) ? 'v' + status.version.replace(/^v/, '').replace(/[-+].*$/, '') : status.version}</span>}
      </div>
      <div className="create-split">
        <button type="button" className="btn primary create-btn" onClick={() => onCreate('item')} title="Add a piece (A)">
          <Icon name="plus" size={15} strokeWidth={2.2} />Add a piece
        </button>
        <button type="button" className="btn primary create-more" aria-label="More ways to start" aria-haspopup="menu" aria-expanded={!!menu} onClick={e => setMenu(menu ? null : rectOf(e.currentTarget))}>
          <Icon name="chevron-down" size={15} />
        </button>
      </div>
      {menu && <Menu anchor={menu} items={createItems(onCreate)} onClose={() => setMenu(null)} width={290} />}

      <nav className="sb-nav" aria-label="Sections">
        {nav.map(n => (
          <NavLink key={n.to} to={n.to} onClick={onNavigate} title={n.hint} className={() => `sb-nav-item ${n.active ? 'active' : ''}`}>
            <Icon name={n.icon} size={16} />{n.label}
            {!!n.count && <span className="sb-count" title={n.hint}>{n.count}</span>}
          </NavLink>
        ))}
      </nav>

      {(trip || summary?.planned_tomorrow === false) && <section className="sb-sec">
        <h4 className="sb-h">Next</h4>
        {summary && !summary.planned_tomorrow && <button type="button" className="sb-row sb-list" onClick={() => { onNavigate(); navigate('/plan') }}>
          <span className="sb-place-ico" aria-hidden="true"><Icon name="calendar-plus" size={14} /></span>
          <span className="grow ellipsis">Plan tomorrow's outfit</span>
        </button>}
        {trip && <button type="button" className={`sb-row sb-list ${path === `/packing/${trip.id}` ? 'active' : ''}`} onClick={() => { onNavigate(); navigate(`/packing/${trip.id}`) }}>
          <span className="sb-place-ico" aria-hidden="true"><Icon name="suitcase" size={14} /></span>
          <span className="grow ellipsis">{trip.name}</span>
          <span className="muted small">{trip.days <= 0 ? 'now' : trip.days === 1 ? 'tomorrow' : fmtDate(trip.start_date, false)}</span>
        </button>}
      </section>}

      <div className="sb-foot">
        <Link to="/settings" className="sb-foot-link" onClick={onNavigate}><Icon name="settings" size={15} />Settings</Link>
        <button type="button" className="sb-foot-link" onClick={() => { onNavigate(); window.dispatchEvent(new CustomEvent('pwdr:shortcuts')) }}><Icon name="keyboard" size={15} />Shortcuts</button>
      </div>
    </aside>
  )
}
