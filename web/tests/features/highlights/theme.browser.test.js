import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'personal highlights, queue, entity overlay and clip tasks follow dark and light themes',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-personal-theme-test',
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/personalTheme.html` })
    const modal = `document.querySelector('[role="dialog"]')`
    await waitFor(`${modal} && ${modal}.textContent.includes('Favorite')`)
    assert.equal(await evaluate(`getComputedStyle(${modal}).backgroundColor`), 'rgb(38, 38, 38)')
    assert.equal(
      await evaluate(`getComputedStyle(${modal}.querySelector('h2')).color`),
      'rgb(242, 242, 242)'
    )
    await evaluate(
      `[...${modal}.querySelectorAll('button')].find(b=>b.textContent==='Rename').click()`
    )
    await waitFor(`${modal}.querySelector('input[aria-label="Highlight name"]')`)
    const input = `${modal}.querySelector('input[aria-label="Highlight name"]')`
    assert.equal(await evaluate(`getComputedStyle(${input}).backgroundColor`), 'rgb(24, 24, 24)')
    assert.equal(await evaluate(`getComputedStyle(${input}).color`), 'rgb(242, 242, 242)')
    await evaluate(`${modal}.querySelector('button[aria-label="Close highlights"]').click()`)
    await waitFor(`document.querySelector('select[aria-label="Highlight filter"]')`)
    await evaluate(
      `[...document.querySelectorAll('button')].find(b=>b.textContent.trim()==='★ Play highlights').click()`
    )
    await waitFor(`${modal}?.getAttribute('aria-label') === 'Play highlights'`)
    assert.equal(await evaluate(`getComputedStyle(${modal}).backgroundColor`), 'rgb(38, 38, 38)')
    assert.equal(
      await evaluate(`getComputedStyle(${modal}.querySelector('select')).color`),
      'rgb(242, 242, 242)'
    )
    await evaluate(`${modal}.querySelector('button[aria-label="Close"]').click()`)
    await waitFor(`!${modal}`)
    await evaluate(
      `[...document.querySelectorAll('nav button')].find(b=>b.textContent==='entity').click()`
    )
    await waitFor(`${modal} && ${modal}.textContent.includes('0 works')`)
    assert.equal(await evaluate(`getComputedStyle(${modal}).backgroundColor`), 'rgb(38, 38, 38)')
    assert.equal(
      await evaluate(`getComputedStyle(${modal}.querySelector('h2')).color`),
      'rgb(242, 242, 242)'
    )
    await evaluate(
      `${modal}.querySelector('button[aria-label="Close related works overlay"]').click()`
    )
    await waitFor(`!${modal}`)
    await evaluate(
      `[...document.querySelectorAll('nav button')].find(b=>b.textContent==='clips').click()`
    )
    await waitFor(`document.querySelector('article')?.textContent.includes('clip.mp4')`)
    assert.equal(
      await evaluate(`getComputedStyle(document.querySelector('article')).backgroundColor`),
      'rgb(38, 38, 38)'
    )
    await evaluate(
      `[...document.querySelectorAll('nav button')].find(b=>b.textContent==='Toggle theme').click()`
    )
    await waitFor(`!document.documentElement.classList.contains('dark')`)
    assert.equal(
      await evaluate(`getComputedStyle(document.querySelector('article')).backgroundColor`),
      'rgb(255, 255, 255)'
    )
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
