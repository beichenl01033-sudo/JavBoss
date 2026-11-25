import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'MPV performance settings preserve defaults, save, discard drafts and do not affect web player settings',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor, reloadPage } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-player-performance-test',
    })
    await command('Page.navigate', { url: `${origin}/tests/fixtures/app.html?view=video` })
    await waitFor(`document.querySelector('aside button[aria-label="Settings"]')`)
    const modal = `document.querySelector('[aria-labelledby="global-settings-title"]')`
    const button = (label) =>
      `[...${modal}.querySelectorAll('button')].find(b=>b.textContent.trim()==='${label}')`
    const seek = `document.querySelector('select[aria-label="MPV seek mode"]')`
    const hwdec = `document.querySelector('select[aria-label="MPV video decoding"]')`
    const open = async () => {
      await evaluate(`document.querySelector('aside button[aria-label="Settings"]').click()`)
      await waitFor(modal)
      await evaluate(`${button('Player')}.click()`)
      await evaluate(`${button('MPV Player')}.click()`)
      await waitFor(seek)
    }
    await open()
    assert.equal(await evaluate(`${seek}.value`), 'adaptive')
    assert.equal(await evaluate(`${hwdec}.value`), 'no')
    await evaluate(
      `${seek}.value='exact';${seek}.dispatchEvent(new Event('change',{bubbles:true}));${hwdec}.value='auto';${hwdec}.dispatchEvent(new Event('change',{bubbles:true}))`
    )
    await evaluate(`${button('Save')}.click()`)
    await waitFor(
      `window.testStore.getState().config.player_seek_mode==='exact' && window.testStore.getState().config.player_hwdec==='auto'`
    )
    assert.equal(
      await evaluate(`window.testStore.getState().config.browser_player_resume_playback`),
      undefined
    )
    await evaluate(
      `${seek}.value='adaptive';${seek}.dispatchEvent(new Event('change',{bubbles:true}));document.querySelector('button[aria-label="Close global settings"]').click()`
    )
    await open()
    assert.equal(await evaluate(`${seek}.value`), 'exact')
    assert.equal(await evaluate(`${hwdec}.value`), 'auto')
    await evaluate(`${button('Restore Defaults')}.click()`)
    assert.equal(await evaluate(`${seek}.value`), 'adaptive')
    assert.equal(await evaluate(`${hwdec}.value`), 'no')
    await evaluate(`${button('Save')}.click()`)
    await waitFor(
      `window.testStore.getState().config.player_seek_mode==='adaptive' && window.testStore.getState().config.player_hwdec==='no'`
    )
    await evaluate(`${button('Web Player')}.click()`)
    assert.equal(await evaluate(`Boolean(${seek})`), false)
    assert.equal(await evaluate(`Boolean(${hwdec})`), false)
    await reloadPage()
    await waitFor(`window.testStore && !window.testStore.getState().loading`)
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
