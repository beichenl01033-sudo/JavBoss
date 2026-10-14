import assert from 'node:assert/strict'
import test from 'node:test'
import {
  VIDEO_SORT_OPTIONS,
  findVideoSortOption,
  normalizeVideoSort,
  reverseVideoSortValue,
  videoSortLabelParts,
} from '../../src/constants/video.js'

test('video watch time sorting supports both directions and replaces play count', () => {
  assert.equal(
    VIDEO_SORT_OPTIONS.some((option) => option.base === 'play_count'),
    false
  )
  const option = findVideoSortOption('watched')
  assert.ok(option)
  assert.equal(findVideoSortOption('watched_asc'), option)
  assert.equal(reverseVideoSortValue('watched'), 'watched_asc')
  assert.equal(reverseVideoSortValue('watched_asc'), 'watched')
  assert.deepEqual(
    videoSortLabelParts(option, 'watched', (cn) => cn),
    {
      label: '观看时长',
      separator: '：',
      direction: '长→短',
    }
  )
})

test('saved video sort preferences and links retain direction when normalized', () => {
  for (const [input, expected] of [
    ['watched', 'watched'],
    ['watched_asc', 'watched_asc'],
    ['watched_desc', 'watched'],
    ['play_count', 'watched'],
    ['play_count_desc', 'watched'],
    [' PLAY_COUNT_ASC ', 'watched_asc'],
  ]) {
    assert.equal(normalizeVideoSort(input), expected)
  }
})

test('video file modified time sorting supports both directions', () => {
  const option = findVideoSortOption('modified')
  assert.ok(option)
  assert.equal(findVideoSortOption('modified_asc'), option)
  assert.equal(normalizeVideoSort('modified_desc'), 'modified')
  assert.equal(reverseVideoSortValue('modified'), 'modified_asc')
  assert.equal(reverseVideoSortValue('modified_asc'), 'modified')
  assert.deepEqual(
    videoSortLabelParts(option, 'modified', (cn) => cn),
    {
      label: '文件修改时间',
      separator: '：',
      direction: '近→远',
    }
  )
})
