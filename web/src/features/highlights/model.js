export function uniqueHighlightVideos(videos = []) {
  const seen = new Set()
  return videos.filter((video) => {
    if (!video?.id || !Number(video.highlight_count) || seen.has(Number(video.id))) return false
    seen.add(Number(video.id))
    return true
  })
}

export function highlightCount(videos) {
  return uniqueHighlightVideos(videos).reduce(
    (total, video) => total + Number(video.highlight_count),
    0
  )
}

// Snapshot filters once, then walk every page in the same deterministic order.
export async function collectHighlightVideos({ fetcher, query, jav = false, signal }) {
  const videos = []
  let offset = 0
  for (;;) {
    if (signal?.aborted) throw new DOMException('Aborted', 'AbortError')
    const result = await fetcher({ ...query, highlightOnly: true, limit: 200, offset, signal })
    const items = result?.items || []
    videos.push(...(jav ? items.flatMap((item) => item.videos || []) : items))
    offset += items.length
    if (!items.length || offset >= Number(result.total)) break
  }
  return uniqueHighlightVideos(videos)
}

export function formatHighlightTime(seconds) {
  const total = Math.max(0, Math.floor(Number(seconds) || 0))
  const minutes = Math.floor(total / 60)
  const rest = String(total % 60).padStart(2, '0')
  return minutes >= 60
    ? `${Math.floor(minutes / 60)}:${String(minutes % 60).padStart(2, '0')}:${rest}`
    : `${minutes}:${rest}`
}
