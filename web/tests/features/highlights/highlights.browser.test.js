import assert from 'node:assert/strict'
import test from 'node:test'
import { browserUnavailable, openBrowser } from '../../helpers/browser.js'

test(
  'highlight badges, edits, shared filter history and cross-page preview queues',
  { skip: browserUnavailable, timeout: 60000 },
  async (t) => {
    const { origin, command, evaluate, waitFor, reloadPage } = await openBrowser(t, {
      cacheDir: 'node_modules/.vite-highlights-test',
    })
    await command('Page.navigate', {
      url: `${origin}/tests/fixtures/app.html?view=video&directory_id=1&highlight_only=1`,
    })
    await waitFor(
      'window.testStore && !window.testStore.getState().loading && document.querySelector("header")'
    )
    await evaluate(`(() => {
    const original = window.fetch;
    window.highlightRequests = [];
    window.highlightDoc = {version:1, revision:'r1',items:[{time:0,label:'Start'},{time:120,label:'Favorite moment'}]};
    window.highlightVideos = [10,11].map(id => ({id,location_id:id+100,filename:'Movie '+id+'.mp4',path:'Movie '+id+'.mp4',directory_id:1,directory:{id:1,path:'/videos'},highlight_count:2}));
    window.highlightCounts = {10:2,11:2};
    window.fetch = async (input,init={}) => {
      const url = new URL(input,location.origin);
      window.highlightRequests.push({url:url.pathname+url.search,method:init.method||'GET',body:init.body&&JSON.parse(init.body)});
      if(url.pathname==='/videos/highlights') return Response.json({counts:window.highlightCounts});
      if(url.pathname==='/videos') return Response.json({items:[window.highlightVideos[Number(url.searchParams.get('offset'))||0]].filter(Boolean),total:2});
      if(url.pathname==='/jav') return Response.json({items:[{id:20,code:'TEST-001',title:'Marked work',videos:window.highlightVideos,tags:[],idols:[]}],total:1});
      if(url.pathname==='/videos/10/highlights') {
        if(init.method==='PATCH') {
          const body=JSON.parse(init.body);
          if(body.label===null) window.highlightDoc.items=window.highlightDoc.items.filter(item=>item.time!==body.time);
          else window.highlightDoc.items.find(item=>item.time===body.time).label=body.label;
          window.highlightDoc.revision+='x';
          window.highlightCounts[10]=window.highlightDoc.items.length;
        }
        return Response.json(window.highlightDoc);
      }
      if(url.pathname==='/videos/11/highlights') return Response.json({version:1,revision:'other',items:[{time:4,label:'Other'}]});
      if(url.pathname==='/videos/playlist') return Response.json({count:JSON.parse(init.body).items.length});
      return original(input,init);
    };
    window.dispatchEvent(new Event('javboss-highlights-changed'));
    window.testStore.getState().loadVideos({force:true});
  })()`)
    const badge = `document.querySelector('button[aria-label="View 2 highlights"]')`
    const modal = `document.querySelector('[role="dialog"][aria-label="Highlights"]')`
    await waitFor(badge)
    assert.equal(await evaluate('window.testStore.getState().highlightOnly'), true)
    await evaluate(`${badge}.click()`)
    await waitFor(`${modal} && ${modal}.textContent.includes('Favorite moment')`)
    await evaluate(`${modal}.querySelector('button[aria-label="Play highlight 0:00"]').click()`)
    await waitFor(`window.highlightRequests.some(r=>r.url==='/videos/playlist')`)
    let request = await evaluate(
      `window.highlightRequests.filter(r=>r.url==='/videos/playlist').at(-1).body`
    )
    assert.equal(request.items[0].highlight_time, 0)
    assert.equal(request.highlight_mode, true)
    await evaluate(
      `[...${modal}.querySelectorAll('button')].find(b=>b.textContent==='Rename').click()`
    )
    await evaluate(
      `(() => {const field=${modal}.querySelector('input[aria-label="Highlight name"]'); Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,'value').set.call(field,'Renamed start'); field.dispatchEvent(new Event('input',{bubbles:true}));})()`
    )
    await evaluate(`${modal}.querySelector('form').requestSubmit()`)
    await waitFor(
      `${modal}.textContent.includes('Renamed start') && !${modal}.querySelector('form')`
    )
    await evaluate(
      `[...${modal}.querySelectorAll('button')].find(b=>b.textContent==='Delete').click()`
    )
    await evaluate(
      `[...${modal}.querySelectorAll('button')].find(b=>b.textContent==='Confirm remove mark').click()`
    )
    await waitFor(`window.highlightDoc.items.length===1`)
    assert.equal(
      await evaluate(`window.highlightRequests.filter(r=>r.method==='PATCH').at(-1).body.label`),
      null
    )
    await evaluate(`${modal}.querySelector('button[aria-label="Close highlights"]').click()`)
    await waitFor(`!${modal}`)
    // Start a queue from the top bar; backend caps responses to one result/page.
    await evaluate(
      `[...document.querySelectorAll('button')].find(b=>b.textContent.trim()==='★ Play highlights').click()`
    )
    const queue = `document.querySelector('[role="dialog"][aria-label="Play highlights"]')`
    await waitFor(queue)
    await evaluate(`${queue}.querySelector('input[type="checkbox"]').click()`)
    await evaluate(
      `[...${queue}.querySelectorAll('button')].find(b=>b.textContent==='Play in MPV').click()`
    )
    await waitFor(`!${queue}`)
    request = await evaluate(
      `window.highlightRequests.filter(r=>r.url==='/videos/playlist').at(-1).body`
    )
    assert.deepEqual(
      request.items.map((item) => item.video_id),
      [10, 11]
    )
    assert.equal(request.preview_seconds, 30)
    const queries = await evaluate(
      `window.highlightRequests.filter(r=>r.url.startsWith('/videos?')).map(r=>r.url)`
    )
    assert.ok(queries.some((url) => new URL(url, origin).searchParams.get('offset') === '1'))
    for (const url of queries) {
      const params = new URL(url, origin).searchParams
      assert.equal(params.get('highlight_only'), '1')
      assert.equal(params.get('directory_id'), '1')
    }
    // Shared filter survives section switches and URL restoration.
    await evaluate(`window.testStore.setState({viewMode:'jav',javTab:'list'})`)
    await waitFor(`location.search.includes('view=jav') && document.querySelector('.jav-card')`)
    assert.equal(new URLSearchParams(await evaluate('location.search')).get('highlight_only'), '1')
    await reloadPage()
    await waitFor(`window.testStore && !window.testStore.getState().javLoading`)
    assert.equal(await evaluate('window.testStore.getState().highlightOnly'), true)
    assert.deepEqual(await evaluate('window.appErrors'), [])
  }
)
