import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'official directory tree is shared across tabs with unmarked filtering and browser history',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor, reloadPage } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-shared-directory-test',
    })
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/app.html?view=jav&tab=idol&directory_id=1&subdirectory=clips&highlight_only=0`,
    })
    await waitFor(
      `window.testStore && document.querySelector('button[aria-label="Select subdirectory"]') && !window.testStore.getState().idolLoading`
    )
    const trigger = `document.querySelector('button[aria-label="Select subdirectory"]')`
    const filter = `document.querySelector('select[aria-label="Highlight filter"]')`
    assert.equal(await evaluate(`${trigger}.title`), '/videos/clips')
    assert.equal(
      await evaluate(
        `Boolean(document.querySelector('select[aria-label="Filter content by directory"]'))`
      ),
      false
    )
    await evaluate(
      `(() => {const original = window.fetch; window.fetch = (input, init) => new URL(input, location.origin).pathname === '/videos/directories' ? Promise.resolve(Response.json({items:[{directory_id:1, directory_path:'/videos',path:'.'},{directory_id:1,directory_path:'/videos',path:'clips'},{directory_id:1,directory_path:'/videos',path:'next'}]})) : original(input,init);})()`
    )
    await evaluate(`${trigger}.click()`)
    const next = `document.querySelector('button[aria-label="Select directory /videos/next"]')`
    await waitFor(next)
    await evaluate(`${next}.click()`)
    await waitFor(
      `window.testStore.getState().videoSubdirectory === 'next' && new URLSearchParams(location.search).get('subdirectory') === 'next'`
    )
    for (const [label, endpoint] of [
      ['Video', '/videos'],
      ['JAV', '/jav'],
      ['Studios', '/jav/studios'],
      ['Series', '/jav/series'],
      ['Idols', '/jav/idols'],
    ]) {
      await evaluate(`document.querySelector('aside button[aria-label="${label}"]').click()`)
      await waitFor(
        `window.requests.some(r=>r.url.startsWith(${JSON.stringify(endpoint + '?')}) && new URL(r.url,location.origin).searchParams.get('subdirectory')==='next')`
      )
      assert.equal(await evaluate(`${trigger}.title`), '/videos/next')
      assert.equal(
        await evaluate(`new URLSearchParams(location.search).get('highlight_only')`),
        '0'
      )
      if (label === 'Video' || label === 'JAV') {
        assert.equal(await evaluate(`${filter}.value`), 'without')
        assert.equal(
          await evaluate(
            `[...document.querySelectorAll('button')].find(b=>b.textContent.trim()==='★ Play highlights').disabled`
          ),
          true
        )
      }
    }
    await reloadPage()
    await waitFor(
      `window.testStore?.getState().videoSubdirectory === 'next' && document.querySelector('aside')`
    )
    assert.equal(await evaluate('window.testStore.getState().highlightExclude'), true)
    await evaluate(`document.querySelector('aside button[aria-label="Video"]').click()`)
    await waitFor(filter)
    await evaluate(
      `(() => {const input=${filter}; input.value='with'; input.dispatchEvent(new Event('change',{bubbles:true}));})()`
    )
    await waitFor(`new URLSearchParams(location.search).get('highlight_only')==='1'`)
    await evaluate('history.back()')
    await waitFor(`window.testStore.getState().highlightExclude && ${filter}.value==='without'`)
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
