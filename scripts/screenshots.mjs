// README screenshots and a browser smoke test, against a demo instance (DEMO_DATA=true seeds a 43-piece wardrobe with four months of wearing):
//
//   DATA_DIR=/tmp/wardrobe-demo DEMO_DATA=true ADMIN_EMAIL=demo@example.com ADMIN_PASSWORD=demodemo1 ADMIN_NAME=Amar PORT=18258 ./pocket-wardrobe
//   CDP=http://127.0.0.1:9348 BASE=http://127.0.0.1:18258 node scripts/screenshots.mjs   # any Chrome with remote debugging
//   SHOTS=0 … node scripts/screenshots.mjs                                                # checks only
// playwright-core comes from web/node_modules, or set PW_FROM to a package.json next to an install of it.
import { createRequire } from 'node:module'
import assert from 'node:assert/strict'

const require = createRequire(process.env.PW_FROM || new URL('../web/package.json', import.meta.url))
let pw
try { pw = require('playwright') } catch { pw = require('playwright-core') }
const { chromium } = pw
const base = process.env.BASE || 'http://127.0.0.1:18258'
const shots = process.env.SHOTS !== '0'
const only = process.env.ONLY || ''
const dir = process.env.OUT || new URL('../docs/screenshots/', import.meta.url).pathname
const browser = process.env.CDP ? await chromium.connectOverCDP(process.env.CDP) : await chromium.launch()
const errors = []

async function session({ phone = false, dark = false } = {}) {
  const ctx = await browser.newContext({
    viewport: phone ? { width: 390, height: 844 } : { width: 1440, height: 900 },
    deviceScaleFactor: phone ? 3 : 2, isMobile: phone, hasTouch: phone, colorScheme: dark ? 'dark' : 'light',
    userAgent: phone ? 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1' : undefined,
  })
  const page = await ctx.newPage()
  page.on('pageerror', e => errors.push(`pageerror: ${e}`))
  page.on('console', m => { if (m.type() === 'error' && !/Failed to load resource|Content Security Policy/.test(m.text())) errors.push(m.text()) })
  await page.goto(base + '/login')
  const res = await page.evaluate(() => fetch('/api/v1/auth/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email: 'demo@example.com', password: 'demodemo1' }) }).then(r => r.status))
  assert.equal(res, 200, 'demo sign-in')
  return { ctx, page }
}
async function go(page, path, wait = 1500) {
  await page.goto(base + path, { waitUntil: 'networkidle' })
  // headless Chrome freezes CSS fade-ins: show everything at once
  await page.addStyleTag({ content: '*, *::before, *::after { transition: none !important; animation: none !important; }' }).catch(() => {})
  await page.evaluate(() => document.fonts.ready)
  await page.waitForTimeout(wait)
}
async function shot(page, name, full = false) { if (shots && (!only || only.split(',').includes(name))) await page.screenshot({ path: dir + name + '.png', fullPage: full }) }
const api = async (page, path) => page.evaluate(p => fetch('/api/v1' + p).then(r => r.json()), path)
const has = async (page, text, what) => assert.ok((await page.content()).includes(text), what)

// ---- desktop ----
{
  const { ctx, page } = await session()
  await go(page, '/', 2500)
  await has(page, 'Wear this', 'Today suggests an outfit')
  await shot(page, 'desktop-today')
  await go(page, '/closet', 1500)
  await has(page, 'Navy blazer', 'the closet lists pieces')
  await shot(page, 'desktop-closet')
  const items = (await api(page, '/items')).items
  const blazer = items.find(i => i.name === 'Navy blazer')
  await go(page, `/closet/${blazer.id}`, 1800)
  await has(page, 'Goes best with', 'a piece shows what goes with it')
  await shot(page, 'desktop-item')
  await go(page, '/outfits', 1500)
  await shot(page, 'desktop-outfits')
  await go(page, '/outfits/ideas', 2000)
  await has(page, 'Outfit ideas', 'ideas page')
  await shot(page, 'desktop-ideas')
  await go(page, '/outfits/new?items=' + items.filter(i => ['Camel coat', 'Grey crew-neck jumper', 'Navy chinos', 'Brown Chelsea boots'].includes(i.name)).map(i => i.id).join(','), 2000)
  await shot(page, 'desktop-maker')
  await go(page, '/shop?style=classic', 2500)
  await shot(page, 'desktop-shop')
  await go(page, '/shop/check', 1200)
  await page.click('text=Check it')
  await page.waitForTimeout(1500)
  await has(page, 'Outfits it would make', 'before you buy shows outfits')
  await shot(page, 'desktop-check')
  await go(page, '/style', 1500)
  await shot(page, 'desktop-style')
  await go(page, '/plan', 3000)
  await shot(page, 'desktop-plan')
  await go(page, '/plan?tab=history', 1500)
  await shot(page, 'desktop-history')
  const packs = (await api(page, '/packs')).packs
  await go(page, `/packing/${packs[0].id}`, 1500)
  await shot(page, 'desktop-packing')
  await go(page, '/stats', 1500)
  await shot(page, 'desktop-stats')
  await go(page, '/settings/apps', 1200)
  await shot(page, 'desktop-settings-apps')
  await ctx.close()
}
// ---- dark ----
{
  const { ctx, page } = await session({ dark: true })
  await go(page, '/', 2500)
  await shot(page, 'desktop-today-dark')
  await ctx.close()
}
// ---- phone ----
{
  const { ctx, page } = await session({ phone: true })
  await go(page, '/', 2500)
  await shot(page, 'phone-today')
  await go(page, '/closet', 1500)
  await shot(page, 'phone-closet')
  const items = (await api(page, '/items')).items
  await go(page, `/closet/${items.find(i => i.name === 'Camel coat').id}`, 1800)
  await shot(page, 'phone-item')
  await go(page, '/shop', 2500)
  await shot(page, 'phone-shop')
  await go(page, '/outfits', 1500)
  await shot(page, 'phone-outfits')
  await go(page, '/stats', 1500)
  await shot(page, 'phone-stats')
  await go(page, '/', 1500)
  await page.click('button[aria-label*="sidebar"], button[aria-label="Show the sidebar"]').catch(() => {})
  await page.waitForTimeout(600)
  await shot(page, 'phone-menu')
  await ctx.close()
}
await browser.close().catch(() => {})
if (errors.length) { console.error(errors.join('\n')); process.exit(1) }
console.log('ok')
