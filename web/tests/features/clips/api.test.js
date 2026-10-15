import assert from 'node:assert/strict'
import test from 'node:test'

import { loadModules } from '../../helpers/modules.js'

test('clip task API lists and controls background exports', async (t) => {
  const [api] = await loadModules(t, ['features/clips/api.js'])
  const requests = []
  t.mock.method(globalThis, 'fetch', async (url, init = {}) => {
    requests.push([url, init.method || 'GET'])
    if (url === '/clips') {
      return Response.json({ items: [], counts: { active: 0, completed: 0, failed: 0 } })
    }
    return new Response(null, { status: 204 })
  })

  await api.fetchClipTasks()
  await api.cancelClipTask(12)
  await api.revealClipTask(12)
  await api.deleteClipTask(12)

  assert.deepEqual(requests, [
    ['/clips', 'GET'],
    ['/clips/12/cancel', 'POST'],
    ['/clips/12/reveal', 'POST'],
    ['/clips/12', 'DELETE'],
  ])
})
