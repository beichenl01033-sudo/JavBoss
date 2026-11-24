export function hasLtxFilename(value) {
  const name = String(value || '')
    .split(/[\\/]/)
    .pop()
  const stem = name.replace(/\.[^.]+$/, '')
  return /(^|[^a-z0-9])ltx(?=$|[^a-z0-9])/i.test(stem)
}

export function ltxCodeKey(value) {
  return String(value || '')
    .trim()
    .toUpperCase()
    .replace(/[-_ ]/g, '')
}

export function suggestLtxFilename(filename, code, enabled) {
  const name = String(filename || '')
    .split(/[\\/]/)
    .pop()
  const ext = name.match(/\.[^.]+$/)?.[0] || ''
  const stem = ext ? name.slice(0, -ext.length) : name
  if (!enabled) {
    return (
      stem
        .replace(/(^|[^a-z0-9])ltx(?=$|[^a-z0-9])/gi, '$1')
        .replace(/[_ .-]+$/, '')
        .replace(/__+/g, '_') + ext
    )
  }
  const normalized = String(code || '')
    .trim()
    .toUpperCase()
  if (!/^[A-Z0-9][A-Z0-9 ._-]{0,127}$/.test(normalized)) throw new Error('请先填写有效的影片番号')
  const part = stem.match(/(?:^|[^a-z0-9])((?:cd|part|disc|disk)[ _-]?\d+)(?=$|[^a-z0-9])/i)?.[1]
  const clip = stem.match(/(?:^|_)(clip[_-].+)$/i)?.[1]
  return `${normalized}_LTX${part ? `_${part.toUpperCase().replace(/[ _-]/g, '')}` : ''}${clip ? `_${clip}` : ''}${ext}`
}

export function catalogState(snapshot, code) {
  if (!snapshot?.complete || !code) return 'unknown'
  const key = ltxCodeKey(code)
  const item = snapshot.items?.find((item) => ltxCodeKey(item.code) === key)
  return item ? (item.pending ? 'pending' : 'confirmed') : 'not_found'
}
