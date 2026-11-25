import { createContext, useContext, useEffect, useState } from 'react'
import { apiFetch, apiError } from '@/api/client'
import { hasLtxFilename, ltxCodeKey } from '@/features/ltx/model'
import { zh } from '@/utils/i18n'
import { useStore, videoSelectionKey } from '@/store'

const CatalogContext = createContext(null)

export function LtxCatalogProvider({ children }) {
  const [catalog, setCatalog] = useState(null)
  useEffect(() => {
    if (!catalog?.updatedAt) return
    if (useStore.getState().ltxCatalog) void useStore.getState().refreshLtxResults()
    window.dispatchEvent(new Event('javboss-ltx-changed'))
  }, [catalog?.updatedAt])
  useEffect(() => {
    let controller
    let lastFetch = 0
    const refresh = async () => {
      if (Date.now() - lastFetch < 30000) return
      lastFetch = Date.now()
      controller?.abort()
      controller = new AbortController()
      const requestController = controller
      try {
        const res = await apiFetch('/ltx/catalog', {
          signal: requestController.signal,
          cache: 'no-store',
        })
        if (!res.ok) throw await apiError(res)
        const data = await res.json()
        if (
          !requestController.signal.aborted &&
          data.complete &&
          Array.isArray(data.items) &&
          data.items.length === data.total
        ) {
          setCatalog({
            updatedAt: data.updated_at,
            index: new Map(data.items.map((item) => [ltxCodeKey(item.code), item])),
          })
        }
      } catch {
        /* Temporary failures preserve the last complete snapshot. */
      }
    }
    refresh()
    window.addEventListener('focus', refresh)
    const timer = window.setInterval(refresh, 60000)
    return () => {
      controller?.abort()
      window.clearInterval(timer)
      window.removeEventListener('focus', refresh)
    }
  }, [])
  return <CatalogContext.Provider value={catalog}>{children}</CatalogContext.Provider>
}

export default function LtxBadges({ code, videos = [], className = '' }) {
  const catalog = useContext(CatalogContext)
  const updates = useStore((state) => state.videoLocationUpdates)
  const localLtx = videos.some((original) => {
    const video = { ...original, ...updates?.[videoSelectionKey(original)] }
    return (
      hasLtxFilename(video.filename || video.path) ||
      video.locations?.some((loc) => {
        const current = updates?.[`loc:${loc.id}`] || loc
        return hasLtxFilename(current.filename || current.relative_path)
      })
    )
  })
  const item = catalog?.index.get(ltxCodeKey(code))
  if (!localLtx && !item) return null
  const href = `https://ltxlib.dpdns.org/?q=${encodeURIComponent(code || '')}`
  return (
    <div className={`flex flex-wrap gap-1 text-[10px] font-semibold ${className}`}>
      {localLtx && (
        <span
          className="rounded bg-emerald-950/90 px-1.5 py-1 text-emerald-100"
          title={zh(
            '本地文件名含 LTX 标记，由用户确认；不代表校验为同一文件',
            'Local filename is labelled LTX; not a file checksum match'
          )}
        >
          {zh('本地 LTX', 'Local LTX')}
        </span>
      )}
      {item && (
        <a
          href={href}
          target="_blank"
          rel="noopener noreferrer"
          onClick={(event) => event.stopPropagation()}
          className={`rounded px-1.5 py-1 ${item.pending ? 'bg-amber-950/90 text-amber-100' : 'bg-indigo-950/90 text-indigo-100'}`}
          title={zh(
            `来自目录缓存，更新于 ${new Date(catalog.updatedAt).toLocaleString()}。仅表示收录状态，不保证资源可用。点击前往新站。`,
            `Cached catalogue, updated ${new Date(catalog.updatedAt).toLocaleString()}; availability is not guaranteed.`
          )}
        >
          {item.pending
            ? zh('LTX 站待确认', 'LTX site: pending')
            : zh('LTX 站已收录', 'LTX site: listed')}
        </a>
      )}
    </div>
  )
}
