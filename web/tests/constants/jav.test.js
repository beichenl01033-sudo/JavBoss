import assert from 'node:assert/strict'
import test from 'node:test'
import {
  JAV_SORT_OPTIONS,
  JAV_ENTITY_OPEN_MODE_MODAL,
  JAV_ENTITY_OPEN_MODE_PAGE,
  IDOL_SORT_OPTIONS,
  findSortOption,
  normalizeIdolSort,
  normalizeJavSort,
  normalizeJavSortRules,
  normalizeJavEntityOpenMode,
  resolveJavSort,
  reverseSortValue,
} from '../../src/constants/jav.js'

test('entity cover open mode defaults to modal and accepts the saved page mode', () => {
  assert.equal(normalizeJavEntityOpenMode(), JAV_ENTITY_OPEN_MODE_MODAL)
  assert.equal(normalizeJavEntityOpenMode(' PAGE '), JAV_ENTITY_OPEN_MODE_PAGE)
  assert.equal(normalizeJavEntityOpenMode('invalid'), JAV_ENTITY_OPEN_MODE_MODAL)
})

test('JAV watch time sorting replaces play count and supports both directions', () => {
  assert.equal(
    JAV_SORT_OPTIONS.some((option) => option.base === 'play_count'),
    false
  )
  const option = findSortOption(JAV_SORT_OPTIONS, 'watched')
  assert.ok(option)
  assert.equal(option.label[0], '观看时长')
  assert.equal(findSortOption(JAV_SORT_OPTIONS, 'watched_asc'), option)
  assert.equal(reverseSortValue(JAV_SORT_OPTIONS, 'watched'), 'watched_asc')
  assert.equal(reverseSortValue(JAV_SORT_OPTIONS, 'watched_asc'), 'watched')
})

test('JAV and idol file modified time sorting supports defaults, rules and both directions', () => {
  const javOption = findSortOption(JAV_SORT_OPTIONS, 'modified')
  assert.ok(javOption)
  assert.equal(javOption.label[0], '文件修改时间')
  assert.equal(findSortOption(JAV_SORT_OPTIONS, 'modified_asc'), javOption)
  assert.equal(normalizeJavSort('modified_desc'), 'modified')
  assert.equal(reverseSortValue(JAV_SORT_OPTIONS, 'modified'), 'modified_asc')

  const rules = [{ id: 'modified', enabled: true, mode: 'all', active: ['idol'], sort: 'modified' }]
  assert.equal(normalizeJavSortRules({ version: 1, rules })[0].sort, 'modified')
  assert.equal(resolveJavSort({ javIdolIds: [1], javSortRules: rules }).sort, 'modified')

  const idolOption = findSortOption(IDOL_SORT_OPTIONS, 'modified')
  assert.ok(idolOption)
  assert.equal(idolOption.label[0], '文件修改时间')
  assert.equal(findSortOption(IDOL_SORT_OPTIONS, 'modified_asc'), idolOption)
  assert.equal(normalizeIdolSort('modified_desc'), 'modified')
  assert.equal(reverseSortValue(IDOL_SORT_OPTIONS, 'modified'), 'modified_asc')
})

test('JAV saved defaults, temporary sorts and automatic rules retain legacy directions', () => {
  for (const [input, expected] of [
    ['watched', 'watched'],
    ['watched_asc', 'watched_asc'],
    ['watched_desc', 'watched'],
    ['play_count', 'watched'],
    ['play_count_desc', 'watched'],
    [' PLAY_COUNT_ASC ', 'watched_asc'],
  ]) {
    assert.equal(normalizeJavSort(input), expected)
    assert.equal(resolveJavSort({ javSort: input }).sort, expected)
    assert.equal(resolveJavSort({ javTempSort: input }).sort, expected)
    const rules = [{ id: 'watch', enabled: true, mode: 'all', active: ['idol'], sort: input }]
    assert.equal(normalizeJavSortRules({ version: 1, rules })[0].sort, expected)
    const resolved = resolveJavSort({ javIdolIds: [1], javSortRules: rules })
    assert.equal(resolved.sort, expected)
    assert.equal(resolved.source, 'rule')
  }
})
