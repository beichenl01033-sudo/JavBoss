import { useCallback, useEffect, useState } from 'react'
import { apiFetch, apiError } from '@/api/client'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

export default function LtxCatalogSettings() {
  const [catalog, setCatalog] = useState(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const refresh = useCallback(async (signal) => {
    setLoading(true)
    try {
      const res = await apiFetch('/ltx/catalog', { cache: 'no-store', signal })
      if (!res.ok) throw await apiError(res)
      const data = await res.json()
      if (signal?.aborted) return
      setCatalog(data)
      setError('')
    } catch (err) {
      if (!signal?.aborted) setError(getErrorMessage(err))
    } finally {
      if (!signal?.aborted) setLoading(false)
    }
  }, [])
  useEffect(() => {
    const controller = new AbortController()
    refresh(controller.signal)
    return () => controller.abort()
  }, [refresh])
  return (
    <section className="space-y-3 rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm dark:border-ui-border dark:bg-ui-surface">
      <h3 className="font-semibold">{zh('LTX 目录同步', 'LTX catalogue sync')}</h3>
      <p className="text-sm text-zinc-600 dark:text-ui-secondary">
        {zh(
          '配置上方 API 令牌后，在装有最新版 JavBoss 助手的浏览器打开新站首页，点击“同步 / 继续同步”。只同步番号与收录状态，不下载视频；同步失败保留旧缓存。',
          'Configure the API token above, open the catalogue in a browser with the latest JavBoss helper, and select Sync. Only movie codes and catalogue status are imported; failed syncs retain the previous cache.'
        )}
      </p>
      <p className="text-sm" role="status">
        {catalog?.complete
          ? zh(
              `缓存 ${catalog.total} 条：已确认 ${catalog.confirmed}，待确认 ${catalog.pending}；更新于 ${new Date(catalog.updated_at).toLocaleString()}`,
              `${catalog.total} cached: ${catalog.confirmed} confirmed, ${catalog.pending} pending; updated ${new Date(catalog.updated_at).toLocaleString()}`
            )
          : zh(
              '尚无完整目录缓存，不能据此判断新站是否收录。',
              'No complete catalogue cache yet; listing status is unknown.'
            )}
      </p>
      {error && (
        <p role="alert" className="text-sm text-red-600">
          {error}
        </p>
      )}
      <div className="flex gap-3 text-sm">
        <a
          className="text-blue-600 underline dark:text-blue-300"
          href="https://ltxlib.dpdns.org/"
          target="_blank"
          rel="noopener noreferrer"
        >
          {zh('打开 LTX 资源库', 'Open LTX catalogue')}
        </a>
        <button
          type="button"
          className="text-blue-600 underline disabled:opacity-40 dark:text-blue-300"
          disabled={loading}
          onClick={() => refresh()}
        >
          {zh('刷新缓存状态', 'Refresh cache status')}
        </button>
      </div>
    </section>
  )
}
