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

test('directory API resolves shared root paths without mixing identical subdirectory names', async (t) => {
  const [{ fetchVideoDirectories }] = await loadModules(t, ['features/video/api.js'])
  const items = [
    { directory_id: 1, path: '.', video_count: 0, videos: [] },
    { directory_id: 1, path: '旅行', video_count: 23, videos: ['one.mp4'] },
    { directory_id: 2, path: '旅行', video_count: 1, videos: ['two.mp4'] },
  ]
  t.mock.method(globalThis, 'fetch', async () =>
    Response.json({
      directory_paths: { 1: '/media/主目录', 2: '/archive' },
      items,
    })
  )
  assert.deepEqual(await fetchVideoDirectories(), [
    { ...items[0], directory_path: '/media/主目录' },
    { ...items[1], directory_path: '/media/主目录' },
    { ...items[2], directory_path: '/archive' },
  ])
})

test('directory API accepts older responses and empty directory lists', async (t) => {
  const [{ fetchVideoDirectories }] = await loadModules(t, ['features/video/api.js'])
  let payload = { items: [{ directory_id: 1, directory_path: '/videos', path: '.' }] }
  t.mock.method(globalThis, 'fetch', async () => Response.json(payload))
  assert.deepEqual(await fetchVideoDirectories(), payload.items)
  payload = { directory_paths: {}, items: [] }
  assert.deepEqual(await fetchVideoDirectories(), [])
})
