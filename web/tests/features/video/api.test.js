import assert from 'node:assert/strict'
import test from 'node:test'

import { loadModules } from '../../helpers/modules.js'

test('MPV playlist requests preserve the selected start index', async (t) => {
  const [api] = await loadModules(t, ['features/video/api.js'])
  let payload
  t.mock.method(globalThis, 'fetch', async (_url, init) => {
    payload = JSON.parse(init.body)
    return Response.json({ count: payload.items.length })
  })

  await api.playVideoPlaylist(
    [
      { video_id: 1, location_id: 11 },
      { video_id: 2, location_id: 22 },
      { video_id: 3, location_id: 33 },
    ],
    'mpv',
    1
  )

  assert.equal(payload.player, 'mpv')
  assert.equal(payload.start_index, 1)
  assert.deepEqual(
    payload.items.map((item) => item.video_id),
    [1, 2, 3]
  )
})
