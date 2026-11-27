import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { homedir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const docsRoot = process.env.GOFORJ_DOCS_DIR || join(homedir(), 'code', 'goforj-docs')
const require = createRequire(join(docsRoot, 'docs', 'package.json'))
const { chromium } = require('playwright')
const assets = new URL('../../../../docs/assets/', import.meta.url)
const output = process.argv[2] || fileURLToPath(new URL('banner.png', assets))
const viewport = { width: 1200, height: 456 }

const browser = await chromium.launch({ channel: 'chrome', headless: true, args: ['--disable-gpu'] })
try {
  const page = await browser.newPage({ viewport, deviceScaleFactor: 2 })
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  for (const file of ['design-system.css', 'favicon-seam-dark.svg']) {
    await page.route(`https://goforj.dev/${file}`, route =>
      route.fulfill({ path: join(docsRoot, 'docs', 'public', file) }),
    )
  }

  await page.goto(new URL('banner.html', assets).href, { waitUntil: 'load' })
  const state = await page.evaluate(async () => {
    const fonts = ['700 124px "Space Grotesk"', '400 22px Inter', '500 18px "JetBrains Mono"']
    const loadedFonts = await Promise.all(fonts.map(async font => (await document.fonts.load(font)).length))
    await document.fonts.ready
    await document.querySelector('.brand img').decode()

    const banner = document.querySelector('.banner')
    const bounds = [...banner.querySelectorAll('.brand, h1, .summary, .growth, .place')].map(element => {
      const { left, right, top, bottom } = element.getBoundingClientRect()
      return { name: element.className || element.tagName, left, right, top, bottom }
    })
    return { loadedFonts, bounds, ground: getComputedStyle(banner).getPropertyValue('--gf-ground-deep') }
  })

  assert.deepEqual(errors, [])
  assert(state.ground.trim(), 'GoForj design tokens did not load')
  assert(state.loadedFonts.every(Boolean), 'Required design fonts did not load')
  for (const { name, left, right, top, bottom } of state.bounds) {
    assert(
      left >= 1 && right <= viewport.width - 1 && top >= 1 && bottom <= viewport.height - 1,
      `${name} extends outside the banner`,
    )
  }
  await page.locator('.banner').screenshot({ path: output })
  console.log(output)
} finally {
  await browser.close()
}
