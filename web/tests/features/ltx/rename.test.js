import test from 'node:test'
import assert from 'node:assert/strict'
import { loadModules } from '../../helpers/modules.js'

test('LTX marking uses a dedicated route while ordinary rename remains compatible', async (t) => {
  const [api] = await loadModules(t, ['features/video/api.js'])
  const requests = []
  t.mock.method(globalThis, 'fetch', async (url, init) => {
    requests.push({ url, method: init.method, body: JSON.parse(init.body) })
    return Response.json({ id: 1, location_id: 2 })
  })
  await api.renameVideoLocation(1, 2, 'ABC-123_LTX.mp4', { ltx: true })
  await api.renameVideoLocation(1, 2, 'ABC-123.mp4', { ltx: false })
  await api.renameVideoLocation(1, 2, 'new-name.mp4')
  assert.deepEqual(requests, [
    {
      url: '/videos/1/locations/2/ltx',
      method: 'PATCH',
      body: { filename: 'ABC-123_LTX.mp4', ltx: true },
    },
    {
      url: '/videos/1/locations/2/ltx',
      method: 'PATCH',
      body: { filename: 'ABC-123.mp4', ltx: false },
    },
    { url: '/videos/1/locations/2', method: 'PATCH', body: { filename: 'new-name.mp4' } },
  ])
})

test('an older backend fails closed without falling back to unsafe video-only rename', async (t) => {
  const [api] = await loadModules(t, ['features/video/api.js'])
  const requests = []
  t.mock.method(globalThis, 'fetch', async (url) => {
    requests.push(url)
    return Response.json(
      { error_zh: '接口不存在', error_en: 'API endpoint was not found' },
      { status: 404 }
    )
  })
  await assert.rejects(() => api.renameVideoLocation(1, 2, 'ABC-123_LTX.mp4', { ltx: true }))
  assert.deepEqual(requests, ['/videos/1/locations/2/ltx'])
})
