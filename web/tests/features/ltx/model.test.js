import test from 'node:test'
import assert from 'node:assert/strict'
import {
  hasLtxFilename,
  suggestLtxFilename,
  catalogState,
} from '../../../src/features/ltx/model.js'

test('LTX is an explicit filename token, never inferred from a directory or partial token', () => {
  for (const name of ['ABC-123_LTX.mp4', 'ABC-123.restored_ltx.mkv', '[LTX] ABC-123.mp4'])
    assert.equal(hasLtxFilename(name), true)
  for (const name of [
    'Z:\\LTX\\ABC-123.mp4',
    '/LTX/ABC-123.mp4',
    'ABC-123_LTX2.mp4',
    'ABC-123_NOTLTX.mp4',
  ])
    assert.equal(hasLtxFilename(name), false)
})
test('suggested renames retain extension, multipart and clip suffixes', () => {
  assert.equal(suggestLtxFilename('original.mp4', 'abc-123', true), 'ABC-123_LTX.mp4')
  assert.equal(suggestLtxFilename('original_CD1.mkv', 'ABC-123', true), 'ABC-123_LTX_CD1.mkv')
  assert.equal(
    suggestLtxFilename('ABC-123_clip_001.mp4', 'ABC-123', true),
    'ABC-123_LTX_clip_001.mp4'
  )
  assert.equal(suggestLtxFilename('ABC-123_LTX_CD1.mp4', '', false), 'ABC-123_CD1.mp4')
  assert.throws(() => suggestLtxFilename('original.mp4', '../bad', true))
})
test('incomplete or missing cache is unknown, not a negative lookup', () => {
  assert.equal(catalogState(null, 'ABC-123'), 'unknown')
  assert.equal(catalogState({ complete: false, items: [] }, 'ABC-123'), 'unknown')
  const catalog = {
    complete: true,
    items: [
      { code: 'ABC-123', pending: false },
      { code: 'DEF-456', pending: true },
    ],
  }
  assert.equal(catalogState(catalog, 'abc_123'), 'confirmed')
  assert.equal(catalogState(catalog, 'DEF456'), 'pending')
  assert.equal(catalogState(catalog, 'ABC-1234'), 'not_found')
})
