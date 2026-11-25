import assert from 'node:assert/strict'
import test from 'node:test'
import { createStore } from 'zustand/vanilla'
import { loadModules } from '../../helpers/modules.js'
import { collectHighlightVideos } from '../../../src/features/highlights/model.js'

test('LTX filters reach every paginated list, filter options, links and highlight queues', async (t) => {
  const [{ createAppState }, queries, urls, links, api, videoApi] = await loadModules(t, [
    'store.js',
    'query/listQueries.js',
    'utils/urlState.js',
    'navigation/queryLinks.js',
    'features/jav/api.js',
    'features/video/api.js',
  ])
  const store = createStore(createAppState)
  store.setState({
    page: 8,
    javPage: 8,
    idolPage: 8,
    studioPage: 8,
    seriesPage: 8,
    selectedDirectoryId: 7,
    videoSubdirectory: 'clips',
    highlightOnly: true,
  })
  store.getState().setLtxFilters({ ltxLocal: 'without', ltxCatalog: 'confirmed' })
  const state = store.getState()
  for (const key of ['page', 'javPage', 'idolPage', 'studioPage', 'seriesPage'])
    assert.equal(state[key], 1)
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
    [queries.javQuery, api.fetchJavFilterOptions],
  ]) {
    await fetcher(query(state))
    const params = requests.at(-1).searchParams
    assert.equal(params.get('ltx_local'), 'without')
    assert.equal(params.get('ltx_catalog'), 'confirmed')
    assert.equal(params.get('directory_id'), '7')
    assert.equal(params.get('subdirectory'), 'clips')
  }
  for (const key of [
    'videoQueryKey',
    'javQueryKey',
    'idolQueryKey',
    'studioQueryKey',
    'seriesQueryKey',
  ]) {
    assert.notEqual(queries[key](state), queries[key]({ ...state, ltxLocal: '' }))
    assert.notEqual(queries[key](state), queries[key]({ ...state, ltxCatalog: 'pending' }))
  }
  for (const tab of ['video', 'list', 'idol', 'studio', 'series']) {
    const normalized = urls.normalizeUrlStateFromStore(
      { ...state, viewMode: tab === 'video' ? 'video' : 'jav', javTab: tab },
      new Map()
    )
    const parsed = urls.parseUrlState(urls.buildUrlFromState(normalized, '/').split('?')[1])
    assert.equal(parsed.ltxLocal, 'without')
    assert.equal(parsed.ltxCatalog, 'confirmed')
    assert.equal(parsed.highlightOnly, true)
  }
  for (const link of [
    links.buildVideoQueryLink(state, {}, '/'),
    links.buildJavQueryLink(state, { tab: 'list' }, '/'),
  ]) {
    assert.equal(new URL(link, 'http://localhost').searchParams.get('ltx_local'), 'without')
  }
  await collectHighlightVideos({
    query: queries.javQuery(state),
    jav: true,
    fetcher: async (query) => {
      assert.equal(query.ltxLocal, 'without')
      assert.equal(query.ltxCatalog, 'confirmed')
      assert.equal(query.highlightOnly, true)
      return { items: [], total: 0 }
    },
  })
  store.getState().setLtxFilters({ ltxLocal: 'anything', ltxCatalog: 'unknown' })
  assert.equal(store.getState().ltxLocal, '')
  assert.equal(store.getState().ltxCatalog, '')
  const invalid = urls.parseUrlState('?ltx_local=bad&ltx_catalog=bad')
  assert.equal(invalid.ltxLocal, '')
  assert.equal(invalid.ltxCatalog, '')
})
