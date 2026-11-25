import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'LTX filters persist across tabs, history, reload and clearing; overlays inherit and combine them with highlights',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor, reloadPage } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-ltx-filter-test',
    })
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/app.html?view=video&ltx_local=without&ltx_catalog=confirmed&highlight_only=1`,
    })
    const local = `document.querySelector('select[aria-label="Local LTX filter"]')`
    const catalog = `document.querySelector('select[aria-label="LTX catalogue filter"]')`
    await waitFor(
      `window.testStore && ${local} && window.requests.some(r=>r.url.startsWith('/videos?') && r.url.includes('ltx_catalog=confirmed')) && !window.testStore.getState().loading`
    )
    assert.equal(await evaluate(`${local}.value`), 'without')
    assert.equal(await evaluate(`${catalog}.value`), 'confirmed')
    for (const [label, endpoint] of [
      ['JAV', '/jav'],
      ['Idols', '/jav/idols'],
      ['Studios', '/jav/studios'],
      ['Series', '/jav/series'],
      ['Video', '/videos'],
    ]) {
      await evaluate(`document.querySelector('aside button[aria-label="${label}"]').click()`)
      await waitFor(
        `window.requests.some(r=>r.url.startsWith('${endpoint}?') && r.url.includes('ltx_local=without') && r.url.includes('ltx_catalog=confirmed'))`
      )
      assert.equal(await evaluate(`${local}.value`), 'without')
      assert.equal(await evaluate(`${catalog}.value`), 'confirmed')
      assert.equal(
        await evaluate(`new URLSearchParams(location.search).get('ltx_local')`),
        'without'
      )
    }
    await evaluate(
      `${local}.value='with'; ${local}.dispatchEvent(new Event('change',{bubbles:true}))`
    )
    await waitFor(`new URLSearchParams(location.search).get('ltx_local')==='with'`)
    await evaluate('history.back()')
    await waitFor(`${local}?.value==='without' && window.testStore.getState().ltxLocal==='without'`)
    await evaluate('history.forward()')
    await waitFor(`${local}?.value==='with' && window.testStore.getState().ltxLocal==='with'`)
    await reloadPage()
    await waitFor(
      `window.testStore && ${local}?.value==='with' && window.testStore.getState().ltxCatalog==='confirmed' && !window.testStore.getState().loading`
    )

    // No server cache is needed by this UI fixture; backend integration tests
    // separately assert 409 on absent/incomplete caches and real SQL membership.
    await evaluate(`(() => { const original=window.fetch; window.fetch=async(input,init)=>{
      if(new URL(input,location.origin).pathname==='/jav/idols') {
        window.requests.push({url:new URL(input,location.origin).pathname+new URL(input,location.origin).search});
        return Response.json({items:[{id:3,name:'Filter idol',cover_code:'ABC-001',work_count:1}],total:1});
      }
      return original(input,init);
    }; window.testStore.setState(s=>({config:{...s.config,jav_entity_open_mode:'modal'}})); })()`)
    await evaluate(`document.querySelector('aside button[aria-label="Idols"]').click()`)
    await waitFor(
      `document.body.textContent.includes('Filter idol') && !window.testStore.getState().idolLoading`
    )
    await evaluate(`document.querySelector('main a[href*="idol_ids=3"]').click()`)
    await waitFor(`document.querySelector('[role="dialog"]')`)
    await waitFor(
      `window.requests.some(r=>r.url.startsWith('/jav?') && r.url.includes('idol_ids=3') && r.url.includes('ltx_local=with') && r.url.includes('ltx_catalog=confirmed'))`
    )
    const modalLocal = `document.querySelector('[role="dialog"] select[aria-label="Local LTX filter"]')`
    await evaluate(
      `${modalLocal}.value='without'; ${modalLocal}.dispatchEvent(new Event('change',{bubbles:true}))`
    )
    await waitFor(
      `window.requests.some(r=>r.url.startsWith('/jav?') && r.url.includes('idol_ids=3') && r.url.includes('ltx_local=without') && r.url.includes('highlight_only=1'))`
    )
    assert.equal(await evaluate('window.testStore.getState().ltxLocal'), 'with')
    await evaluate(
      `document.querySelector('button[aria-label="Close related works overlay"]').click()`
    )
    await evaluate(`document.querySelector('aside button[aria-label="JAV"]').click()`)
    await waitFor(
      `document.querySelector('.filter-clear-button') && !window.testStore.getState().javLoading`
    )
    await evaluate(
      `[...document.querySelectorAll('.filter-clear-button')].find(b=>b.textContent.trim()==='Clear').click()`
    )
    await waitFor(
      `${local}.value==='' && ${catalog}.value==='' && !new URLSearchParams(location.search).has('ltx_catalog')`
    )
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
