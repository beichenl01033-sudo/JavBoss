import { apiFetch, apiError, jsonHeaders } from '@/api/client'

export async function fetchHighlightCounts({ signal } = {}) {
  const res = await apiFetch('/videos/highlights', { signal, cache: 'no-store' })
  if (!res.ok) throw await apiError(res)
  return (await res.json()).counts || {}
}

export async function fetchHighlights(id, { signal } = {}) {
  const res = await apiFetch(`/videos/${id}/highlights`, { signal, cache: 'no-store' })
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export async function editHighlight(id, { time, label, revision }) {
  const res = await apiFetch(`/videos/${id}/highlights`, {
    method: 'PATCH',
    headers: jsonHeaders,
    body: JSON.stringify({ time, label, revision }),
  })
  if (!res.ok) throw await apiError(res)
  return res.json()
}

export async function playHighlights(videos, { previewSeconds = 0, leadSeconds = 0, signal } = {}) {
  const res = await apiFetch('/videos/playlist', {
    signal,
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({
      player: 'mpv',
      highlight_mode: true,
      preview_seconds: previewSeconds,
      lead_seconds: leadSeconds,
      items: videos.map((video) => ({
        video_id: video.id,
        location_id: video.location_id || 0,
        highlight_time: video.highlight_time,
      })),
    }),
  })
  if (!res.ok) throw await apiError(res)
  return res.json()
}
