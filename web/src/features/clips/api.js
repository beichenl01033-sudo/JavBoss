import { apiFetch, apiError, parseJSONResponse } from '@/api/client'

export async function fetchClipTasks({ signal } = {}) {
  const res = await apiFetch('/clips', { cache: 'no-store', signal })
  if (!res.ok) throw await apiError(res)
  return parseJSONResponse(res)
}

export async function cancelClipTask(id) {
  const res = await apiFetch(`/clips/${encodeURIComponent(id)}/cancel`, { method: 'POST' })
  if (!res.ok) throw await apiError(res)
}

export async function revealClipTask(id) {
  const res = await apiFetch(`/clips/${encodeURIComponent(id)}/reveal`, { method: 'POST' })
  if (!res.ok) throw await apiError(res)
}

export async function deleteClipTask(id) {
  const res = await apiFetch(`/clips/${encodeURIComponent(id)}`, { method: 'DELETE' })
  if (!res.ok) throw await apiError(res)
}
