import assert from 'node:assert/strict'
import test from 'node:test'
import { createStore } from 'zustand/vanilla'
import { loadModules } from '../../helpers/modules.js'
import { collectHighlightVideos } from '../../../src/features/highlights/model.js'

test('shared subdirectory and unmarked filter reach every list and survive URLs', async (t) => {
  const [{ createAppState }, queries, urls, api, videoApi] = await loadModules(t, [
    'store.js',
    'query/listQueries.js',
    'utils/urlState.js',
    'features/jav/api.js',
    'features/video/api.js',
  ])
  const store = createStore(createAppState)
  store.setState({ page: 3, javPage: 4, idolPage: 5, studioPage: 6, seriesPage: 7 })
  store.getState().setHighlightFilter('without')
  store.getState().setVideoDirectoryFilter({ directory_id: 7, path: "旅行/100%_fun's" })
  const state = store.getState()
  for (const key of ['page', 'javPage', 'idolPage', 'studioPage', 'seriesPage'])
    assert.equal(state[key], 1)
  assert.equal(state.highlightExclude, true)
  assert.equal(state.highlightOnly, false)
  const requests = []
  t.mock.method(globalThis, 'fetch', async (url) => {
    requests.push(new URL(url, 'http://localhost'))
    return Response.json({ items: [], total: 0 })
  })
  for (const [query, fetcher] of [
    [queries.videoQuery, videoApi.fetchVideos],
    [queries.javQuery, api.fetchJavs],
    [queries.idolQuery, api.fetchJavIdols],
    [queries.studioQuery, api.fetchJavStudios],
    [queries.seriesQuery, api.fetchJavSeries],
  ]) {
    const params = query(state)
    assert.equal(params.directoryId, 7)
    assert.equal(params.subdirectory, "旅行/100%_fun's")
    await fetcher(params)
    const search = requests.at(-1).searchParams
    assert.equal(search.get('directory_id'), '7')
    assert.equal(search.get('subdirectory'), "旅行/100%_fun's")
  }
  assert.equal(requests[0].searchParams.get('highlight_only'), '0')
  assert.equal(requests[1].searchParams.get('highlight_only'), '0')
  await api.fetchJavFilterOptions(queries.javQuery(state))
  assert.equal(requests.at(-1).searchParams.get('highlight_only'), '0')
  for (const viewMode of ['video', 'jav']) {
    const normalized = urls.normalizeUrlStateFromStore({ ...state, viewMode }, new Map())
    const url = urls.buildUrlFromState(normalized, '/')
    const parsed = urls.parseUrlState(url.split('?')[1])
    assert.equal(parsed.video.subdirectory, "旅行/100%_fun's")
    assert.equal(parsed.directoryId, 7)
    assert.equal(parsed.highlightExclude, true)
    assert.equal(parsed.highlightOnly, false)
  }
  for (const key of [
    'videoQueryKey',
    'javQueryKey',
    'idolQueryKey',
    'studioQueryKey',
    'seriesQueryKey',
  ]) {
    assert.notEqual(queries[key](state), queries[key]({ ...state, videoSubdirectory: 'other' }))
  }
  for (const key of ['videoQueryKey', 'javQueryKey']) {
    assert.notEqual(queries[key](state), queries[key]({ ...state, highlightExclude: false }))
  }
  store.getState().setHighlightFilter('with')
  assert.equal(store.getState().highlightOnly, true)
  assert.equal(store.getState().highlightExclude, false)
  store.getState().setHighlightFilter('all')
  assert.equal(store.getState().highlightOnly, false)
  assert.equal(store.getState().highlightExclude, false)
})

test('highlight queues never send conflicting unmarked filters and retain subdirectories', async () => {
  await collectHighlightVideos({
    query: { directoryId: 7, subdirectory: 'clips', highlightExclude: true },
    fetcher: async (query) => {
      assert.equal(query.highlightOnly, true)
      assert.equal(query.highlightExclude, false)
      assert.equal(query.directoryId, 7)
      assert.equal(query.subdirectory, 'clips')
      return { items: [], total: 0 }
    },
  })
})
