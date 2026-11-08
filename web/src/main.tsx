import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom'
import './styles/base.css'
import './styles/wardrobe.css'
import './styles/app.css'
import './styles/settings.css'
import { AppProvider, useApp } from './lib/app-context'
import { Loading, ToastProvider } from './components/ui'
import Layout from './components/Layout'
import { ErrorBoundary } from './components/ErrorBoundary'
import Login from './pages/Login'
import Today from './pages/Today'
import Closet from './pages/Closet'
import ItemPage from './pages/ItemPage'
import Outfits, { OutfitIdeas, OutfitMaker, OutfitPage } from './pages/Outfits'
import Plan from './pages/Plan'
import StylePage from './pages/Style'
import Shop from './pages/Shop'
import Packing, { PackPage } from './pages/Packing'
import Laundry from './pages/Laundry'
import Stats from './pages/Stats'
import Settings from './pages/Settings'

function Gate({ children }: { children: React.ReactNode }) {
  const { status, user } = useApp()
  const loc = useLocation()
  if (!status) return <div className="boot"><img src="/favicon.svg" alt="" /><Loading /></div>
  if (status.needs_setup) return <Navigate to="/setup" replace />
  if (!user) return <Navigate to="/login" replace state={{ from: loc.pathname + loc.search }} />
  return <>{children}</>
}

function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login mode="login" />} />
      <Route path="/setup" element={<Login mode="setup" />} />
      <Route path="/settings/:tab?" element={<Gate><ErrorBoundary where="settings"><Settings /></ErrorBoundary></Gate>} />
      <Route element={<Gate><ErrorBoundary where="wardrobe"><Layout /></ErrorBoundary></Gate>}>
        <Route path="/" element={<Today />} />
        <Route path="/closet" element={<Closet />} />
        <Route path="/closet/:id" element={<ItemPage />} />
        <Route path="/outfits" element={<Outfits />} />
        <Route path="/outfits/new" element={<OutfitMaker />} />
        <Route path="/outfits/ideas" element={<OutfitIdeas />} />
        <Route path="/outfits/:id" element={<OutfitPage />} />
        <Route path="/outfits/:id/edit" element={<OutfitMaker />} />
        <Route path="/plan" element={<Plan />} />
        <Route path="/style" element={<StylePage />} />
        <Route path="/shop/:tab?" element={<Shop />} />
        <Route path="/packing" element={<Packing />} />
        <Route path="/packing/:id" element={<PackPage />} />
        <Route path="/laundry" element={<Laundry />} />
        <Route path="/stats" element={<Stats />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <ErrorBoundary where="app"><ToastProvider><AppProvider><App /></AppProvider></ToastProvider></ErrorBoundary>
    </BrowserRouter>
  </React.StrictMode>,
)
