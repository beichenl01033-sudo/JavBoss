import assert from 'node:assert/strict'
import test from 'node:test'
import { collectHighlightVideos, highlightCount } from '../../../src/features/highlights/model.js'

test('highlight queues retain filters and order across pages and deduplicate content locations', async () => {
  const requests = []
  const result = await collectHighlightVideos({
    jav: true,
    query: { directoryId: 7, idolIds: [9], sort: 'random', seed: 42 },
    fetcher: async (query) => {
      requests.push(query)
      return {
        total: 2,
        items: [
          {
            videos:
              query.offset === 0
                ? [
                    { id: 2, location_id: 12, highlight_count: 3 },
                    { id: 2, location_id: 13, highlight_count: 3 },
                  ]
                : [
                    { id: 3, location_id: 14, highlight_count: 1 },
                    { id: 4, highlight_count: 0 },
                  ],
          },
        ],
      }
    },
  })
  assert.deepEqual(
    result.map((item) => [item.id, item.location_id]),
    [
      [2, 12],
      [3, 14],
    ]
  )
  assert.equal(requests.length, 2)
  for (const query of requests) {
    assert.equal(query.directoryId, 7)
    assert.deepEqual(query.idolIds, [9])
    assert.equal(query.highlightOnly, true)
    assert.equal(query.seed, 42)
  }
  assert.equal(
    highlightCount([
      { id: 2, highlight_count: 3 },
      { id: 2, highlight_count: 3 },
    ]),
    3
  )
})

test('closing a queue prevents further page requests', async () => {
  const controller = new AbortController()
  let calls = 0
  await assert.rejects(
    collectHighlightVideos({
      query: {},
      signal: controller.signal,
      fetcher: async () => {
        calls += 1
        controller.abort()
        return { total: 20, items: [{ id: 1, highlight_count: 2 }] }
      },
    }),
    { name: 'AbortError' }
  )
  assert.equal(calls, 1)
})
