// The app shell: sidebar, top bar, search palette, shortcuts — the Pocket Calendar layout.
import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { DataProvider, useData } from '../lib/data'
import { Icon } from './icons'
import { Sidebar } from './Sidebar'
import { CommandPalette, type PaletteCommand } from './CommandPalette'
import { ShortcutsSheet } from './ShortcutsSheet'
import { ItemForm, ManyFromPhoto, PackForm } from './Forms'
import { Avatar, Loading, type MenuItem, MenuButton, modalOpen, useIsMobile, useMediaQuery } from './ui'
import { AskButton } from './AskButton'

export { announceChange, useOnChange } from '../lib/data'

const SIDEBAR_KEY = 'pwdr-sidebar'

/** The page's own name for the top bar breadcrumb. */
const CrumbCtx = createContext<(t: string) => void>(() => {})
export function useCrumb(title: string | undefined) {
  const set = useContext(CrumbCtx)
  useEffect(() => { set(title || ''); return () => set('') }, [title, set])
}

export type CreateKind = 'item' | 'many' | 'outfit' | 'check' | 'pack' | 'look'
export type CreateInitial = Record<string, unknown>

/** Start something from anywhere (a page's button), with what is known already. */
export const openCreate = (k: CreateKind, initial?: CreateInitial) => window.dispatchEvent(new CustomEvent('pwdr:create', { detail: { kind: k, initial } }))

/** The "new" menu: the sidebar's caret. */
export function createItems(onCreate: (k: CreateKind) => void): MenuItem[] {
  return [
    { label: 'Add a piece', icon: 'camera', hint: 'A', onSelect: () => onCreate('item') },
    { label: 'Several pieces from one photo', icon: 'image', onSelect: () => onCreate('many') },
    'sep',
    { label: 'Make an outfit', icon: 'layers', hint: 'O', onSelect: () => onCreate('outfit') },
    { label: 'Rate my look (mirror photo)', icon: 'mirror', onSelect: () => onCreate('look') },
    'sep',
    { label: 'Should I buy this?', icon: 'cart', hint: 'B', onSelect: () => onCreate('check') },
    { label: 'Pack for a trip', icon: 'suitcase', hint: 'P', onSelect: () => onCreate('pack') },
  ]
}

export default function Layout() {
  return <DataProvider><Shell /></DataProvider>
}

function Shell() {
  const navigate = useNavigate()
  const loc = useLocation()
  const { summary } = useData()
  const mobile = useIsMobile()
  const narrow = useMediaQuery('(max-width: 1099px)')
  const drawer = mobile || narrow
  const [sidebarOpen, setSidebarOpen] = useState(() => !drawer && localStorage.getItem(SIDEBAR_KEY) !== '0')
  useEffect(() => { setSidebarOpen(!drawer && localStorage.getItem(SIDEBAR_KEY) !== '0') }, [drawer])
  const drawerOpenedAt = useRef(0)
  const toggleSidebar = () => setSidebarOpen(o => { drawerOpenedAt.current = performance.now(); const n = !o; if (!drawer) localStorage.setItem(SIDEBAR_KEY, n ? '1' : '0'); return n })
  const closeDrawer = () => { if (drawer) setSidebarOpen(false) }
  useEffect(() => { document.querySelector('.content')?.scrollTo({ top: 0 }) }, [loc.pathname])

  const [crumb, setCrumbRaw] = useState('')
  const setCrumb = useCallback((t: string) => setCrumbRaw(t), [])

  const [palette, setPalette] = useState(false)
  const [shortcuts, setShortcuts] = useState(false)
  const [making, setMaking] = useState<{ kind: CreateKind; initial: CreateInitial } | null>(null)

  const create = (k: CreateKind, initial?: CreateInitial) => {
    closeDrawer()
    if (k === 'outfit') { navigate('/outfits/new'); return }
    if (k === 'check') { navigate('/shop/check'); return }
    if (k === 'look') { navigate('/style#look'); return }
    setMaking({ kind: k, initial: initial || {} })
  }
  const createRef = useRef(create)
  createRef.current = create

  // /?add=item (the PWA shortcut) opens the dialog
  useEffect(() => {
    const p = new URLSearchParams(loc.search)
    const a = p.get('add')
    if (a === 'item' || a === 'many' || a === 'pack' || a === 'outfit' || a === 'check') { createRef.current(a); navigate(loc.pathname, { replace: true }) }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    const open = () => setShortcuts(true)
    const make = (e: Event) => { const d = (e as CustomEvent<{ kind: CreateKind; initial?: CreateInitial }>).detail; createRef.current(d.kind, d.initial) }
    window.addEventListener('pwdr:shortcuts', open)
    window.addEventListener('pwdr:create', make)
    return () => { window.removeEventListener('pwdr:shortcuts', open); window.removeEventListener('pwdr:create', make) }
  }, [])
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') { e.preventDefault(); setPalette(p => !p); return }
      if (e.metaKey || e.ctrlKey || e.altKey) return
      if (t.closest('input, textarea, select, [contenteditable="true"]') || modalOpen() || document.querySelector('.menu, .popover, .sel-pop')) return
      const go = (to: string) => { e.preventDefault(); navigate(to) }
      const mk = (k: CreateKind) => { e.preventDefault(); createRef.current(k) }
      switch (e.key) {
        case '/': e.preventDefault(); setPalette(true); break
        case '?': e.preventDefault(); setShortcuts(true); break
        case 'a': mk('item'); break
        case 'o': mk('outfit'); break
        case 'b': mk('check'); break
        case 'p': mk('pack'); break
        case 't': go('/'); break
        case 'c': go('/closet'); break
        case 'f': go('/outfits'); break
        case 'w': go('/plan'); break
        case 's': go('/shop'); break
        case 'y': go('/style'); break
        case 'l': go('/laundry'); break
        case ',': go('/settings'); break
        case '[': e.preventDefault(); toggleSidebar(); break
      }
    }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }) // eslint-disable-line react-hooks/exhaustive-deps

  const commands: PaletteCommand[] = [
    { id: 'item', label: 'Add a piece (photo)', icon: 'camera', hint: 'A', keywords: 'new clothes shirt shoes photo', run: () => create('item') },
    { id: 'many', label: 'Several pieces from one photo', icon: 'image', keywords: 'bulk rail bed many ai', run: () => create('many') },
    { id: 'outfit', label: 'Make an outfit', icon: 'layers', hint: 'O', keywords: 'combine look put together', run: () => create('outfit') },
    { id: 'check', label: 'Should I buy this?', icon: 'cart', hint: 'B', keywords: 'shop buy check before', run: () => create('check') },
    { id: 'pack', label: 'Pack for a trip', icon: 'suitcase', hint: 'P', keywords: 'travel holiday suitcase capsule', run: () => create('pack') },
    { id: 'today', label: 'Today: what to wear', icon: 'today', hint: 'T', run: () => navigate('/') },
    { id: 'closet', label: 'Closet', icon: 'hanger', hint: 'C', keywords: 'wardrobe clothes pieces items', run: () => navigate('/closet') },
    { id: 'outfits', label: 'Outfits', icon: 'layers', hint: 'F', keywords: 'saved looks', run: () => navigate('/outfits') },
    { id: 'ideas', label: 'Outfit ideas for a day, a style, an occasion', icon: 'shuffle', keywords: 'generate combinations mix', run: () => navigate('/outfits/ideas') },
    { id: 'plan', label: 'Plan and history', icon: 'calendar', hint: 'W', keywords: 'week worn log calendar', run: () => navigate('/plan') },
    { id: 'style', label: 'Your style and colours', icon: 'palette', hint: 'Y', keywords: 'profile colour analysis season', run: () => navigate('/style') },
    { id: 'shop', label: 'Worth buying', icon: 'cart', hint: 'S', keywords: 'shopping ideas gaps wishlist', run: () => navigate('/shop') },
    { id: 'bought', label: 'Clothes you bought (Money, Parcels)', icon: 'receipt', keywords: 'purchases import returns', run: () => navigate('/shop/bought') },
    { id: 'packing', label: 'Packing lists', icon: 'suitcase', keywords: 'trips travel', run: () => navigate('/packing') },
    { id: 'laundry', label: 'Laundry and the tailor', icon: 'washer', hint: 'L', keywords: 'wash repair lent', run: () => navigate('/laundry') },
    { id: 'stats', label: 'Your wardrobe in numbers', icon: 'chart', keywords: 'stats cost per wear never worn', run: () => navigate('/stats') },
    { id: 'settings', label: 'Settings', icon: 'settings', hint: ',', run: () => navigate('/settings') },
    { id: 'apps', label: 'Settings: connected apps', icon: 'link', keywords: 'weather calendar travel trails fitness wishlist money parcels tasks chat notes core', run: () => navigate('/settings/apps') },
    { id: 'ai-set', label: 'Settings: AI', icon: 'sparkles', keywords: 'pocket ai openai photos', run: () => navigate('/settings/ai') },
    { id: 'export', label: 'Settings: export', icon: 'download', keywords: 'backup json csv', run: () => navigate('/settings/data') },
    { id: 'shortcuts', label: 'Keyboard shortcuts', icon: 'keyboard', hint: '?', run: () => setShortcuts(true) },
  ]

  const close = () => setMaking(null)
  const m = making
  return (
    <CrumbCtx.Provider value={setCrumb}>
      <div className={`app ${sidebarOpen ? 'with-sidebar' : 'no-sidebar'} ${mobile ? 'is-mobile' : ''} ${drawer ? 'drawer-mode' : ''}`}>
        {drawer && sidebarOpen && <div className="drawer-backdrop" onClick={e => { if (e.timeStamp - drawerOpenedAt.current > 400) setSidebarOpen(false) }} />}
        <div className={`sidebar-wrap ${sidebarOpen ? 'open' : ''}`}>
          <Sidebar onCreate={create} onNavigate={closeDrawer} />
        </div>
        <div className="main">
          <TopBar onMenu={toggleSidebar} onPalette={() => setPalette(true)} mobile={mobile} compact={narrow} sidebarOpen={sidebarOpen} crumb={crumb} />
          <main className="content"><div className="content-inner">{summary ? <Outlet /> : <Loading />}</div></main>
        </div>
        {mobile && !/^\/(settings|outfits\/new)/.test(loc.pathname) && <button type="button" className="fab" aria-label="Add a piece" title="Add a piece" onClick={() => create('item')}><Icon name="plus" size={24} strokeWidth={2.2} /></button>}
        {palette && <CommandPalette onClose={() => setPalette(false)} commands={commands} />}
        {shortcuts && <ShortcutsSheet onClose={() => setShortcuts(false)} />}
        {m?.kind === 'item' && <ItemForm initial={m.initial} onClose={close} />}
        {m?.kind === 'many' && <ManyFromPhoto onClose={close} />}
        {m?.kind === 'pack' && <PackForm onClose={close} />}
      </div>
    </CrumbCtx.Provider>
  )
}

const SECTION: Record<string, [string, string]> = {
  '': ['Today', '/'], closet: ['Closet', '/closet'], outfits: ['Outfits', '/outfits'], plan: ['Plan', '/plan'], style: ['Your style', '/style'], shop: ['Worth buying', '/shop'],
  packing: ['Packing', '/packing'], laundry: ['Laundry', '/laundry'], stats: ['In numbers', '/stats'],
}

function TopBar({ onMenu, onPalette, mobile, compact, sidebarOpen, crumb }: { onMenu: () => void; onPalette: () => void; mobile: boolean; compact: boolean; sidebarOpen: boolean; crumb: string }) {
  const { user, themeMode, setThemeMode, status, setUser } = useApp()
  const navigate = useNavigate()
  const loc = useLocation()
  const [section, sectionTo] = SECTION[loc.pathname.split('/')[1] || ''] || ['', '/']
  const signOut = async () => { await api.post(`${V1}/auth/logout`).catch(() => {}); setUser(null); navigate('/login') }
  const avatarMenu = (): MenuItem[] => [
    { heading: user?.name || user?.email || '' },
    { label: 'Profile', icon: 'user', onSelect: () => navigate('/settings/profile') },
    { label: 'Settings', icon: 'settings', onSelect: () => navigate('/settings'), hint: ',' },
    { label: 'Keyboard shortcuts', icon: 'keyboard', onSelect: () => window.dispatchEvent(new CustomEvent('pwdr:shortcuts')), hint: '?' },
    'sep',
    { heading: 'Appearance' },
    { label: 'System', checked: themeMode === 'system', onSelect: () => setThemeMode('system') },
    { label: 'Light', checked: themeMode === 'light', onSelect: () => setThemeMode('light') },
    { label: 'Dark', checked: themeMode === 'dark', onSelect: () => setThemeMode('dark') },
    ...(status?.auth_mode === 'local' ? ['sep' as const, { label: 'Sign out', icon: 'log-out', onSelect: signOut }] : []),
  ]
  return (
    <header className="topbar">
      <button type="button" className="btn ghost icon-only" onClick={onMenu} aria-label={sidebarOpen ? 'Hide the sidebar' : 'Show the sidebar'} title="Menu ([)"><Icon name={mobile || compact ? 'menu' : 'sidebar'} size={18} /></button>
      <h1 className="range-title">
        {crumb ? <><button type="button" className="crumb" onClick={() => navigate(sectionTo)}>{section}</button><Icon name="chevron-right" size={16} className="crumb-sep" /><span className="ellipsis">{crumb}</span></> : section}
      </h1>
      <span className="spacer" />
      <button type="button" className="btn ghost icon-only" onClick={onPalette} aria-label="Search" title="Search (⌘K or /)"><Icon name="search" size={18} /></button>
      <AskButton />
      <MenuButton label="Account" className="avatar-btn" align="right" width={220} items={avatarMenu}>
        <Avatar name={user?.name} email={user?.email} size={30} />
      </MenuButton>
    </header>
  )
}
