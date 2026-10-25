import { useEffect, useState } from 'react'
import CancelOutlinedIcon from '@mui/icons-material/CancelOutlined'
import DeleteOutlineOutlinedIcon from '@mui/icons-material/DeleteOutlineOutlined'
import FolderOpenOutlinedIcon from '@mui/icons-material/FolderOpenOutlined'
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined'
import RefreshOutlinedIcon from '@mui/icons-material/RefreshOutlined'
import Tooltip from '@/shared/ui/Tooltip'
import { useStore } from '@/store'
import { configFlag } from '@/utils/config'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'
import {
  cancelClipTask,
  deleteClipTask,
  fetchClipTasks,
  revealClipTask,
} from '@/features/clips/api'

const terminalStatuses = new Set(['completed', 'failed', 'canceled'])

function statusLabel(item) {
  if (item?.status === 'running') {
    return (
      {
        reading_network_source: zh('从网络盘读取', 'Reading network source'),
        writing_network_output: zh('写回网络盘', 'Writing to network drive'),
        clipping: zh('正在剪辑', 'Clipping'),
      }[item?.phase] || zh('正在剪辑', 'Clipping')
    )
  }
  return (
    {
      completed: zh('已完成', 'Completed'),
      failed: zh('失败', 'Failed'),
      canceled: zh('已取消', 'Canceled'),
    }[item?.status] || item?.status
  )
}

function formatSeconds(value) {
  const total = Math.max(0, Math.round(Number(value) || 0))
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const seconds = total % 60
  if (hours > 0) {
    return [hours, minutes, seconds].map((part) => String(part).padStart(2, '0')).join(':')
  }
  return [minutes, seconds].map((part) => String(part).padStart(2, '0')).join(':')
}

function formatTime(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString()
}

export default function ClipTasksView({ onToast }) {
  const containerMode = useStore((state) => configFlag(state.config?.runtime_container))
  const [items, setItems] = useState([])
  const [counts, setCounts] = useState({ active: 0, completed: 0, failed: 0 })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [refreshVersion, setRefreshVersion] = useState(0)
  const [busyIds, setBusyIds] = useState(() => new Set())

  useEffect(() => {
    const controller = new AbortController()
    let timer
    const load = async () => {
      try {
        const payload = await fetchClipTasks({ signal: controller.signal })
        if (controller.signal.aborted) return
        setItems(Array.isArray(payload?.items) ? payload.items : [])
        setCounts(payload?.counts || { active: 0, completed: 0, failed: 0 })
        setError('')
      } catch (loadError) {
        if (!controller.signal.aborted) setError(getErrorMessage(loadError))
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false)
          timer = window.setTimeout(load, 1000)
        }
      }
    }
    void load()
    return () => {
      controller.abort()
      window.clearTimeout(timer)
    }
  }, [refreshVersion])

  const runAction = async (item, action, successMessage) => {
    setBusyIds((current) => new Set(current).add(item.id))
    setError('')
    try {
      await action(item.id)
      if (successMessage) onToast?.(successMessage)
      setRefreshVersion((current) => current + 1)
    } catch (actionError) {
      setError(getErrorMessage(actionError))
    } finally {
      setBusyIds((current) => {
        const next = new Set(current)
        next.delete(item.id)
        return next
      })
    }
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto px-4 pb-4 pt-3 sm:px-5 sm:pb-5">
      {error ? (
        <div role="alert" className="mb-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
          {error}
        </div>
      ) : null}

      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-sm text-gray-500">
          {zh(
            `进行中 ${counts.active} · 已完成 ${counts.completed} · 失败 ${counts.failed}`,
            `${counts.active} active · ${counts.completed} completed · ${counts.failed} failed`
          )}
        </p>
        <button
          type="button"
          onClick={() => setRefreshVersion((current) => current + 1)}
          aria-label={zh('刷新剪辑任务', 'Refresh clip tasks')}
          title={zh('刷新剪辑任务', 'Refresh clip tasks')}
          className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-gray-200 bg-white text-[16px] text-gray-600 hover:bg-gray-50 hover:text-gray-900"
        >
          <RefreshOutlinedIcon fontSize="inherit" />
        </button>
      </div>

      {loading ? (
        <div className="mt-4 rounded-xl border border-dashed border-gray-200 bg-white p-10 text-center text-sm text-gray-400">
          {zh('加载剪辑任务…', 'Loading clip tasks...')}
        </div>
      ) : items.length === 0 ? (
        <div className="mt-4 rounded-xl border border-dashed border-gray-200 bg-white p-10 text-center text-sm text-gray-400">
          {zh(
            '暂无剪辑任务。在 MPV 中选定开始点和结束点后提交即可。',
            'No clip tasks yet. Choose start and end points in MPV, then submit the clip.'
          )}
        </div>
      ) : (
        <div className="mt-3 space-y-2">
          {items.map((item) => {
            const progress = Math.max(0, Math.min(100, Number(item.progress) || 0))
            const terminal = terminalStatuses.has(item.status)
            const busy = busyIds.has(item.id)
            return (
              <article
                key={item.id}
                className="rounded-lg border border-gray-200 bg-white p-3 shadow-sm"
              >
                <div className="flex flex-wrap items-start gap-2">
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-1.5">
                      <span className="truncate text-sm font-semibold text-gray-900">
                        {item.output_filename || item.source_filename}
                      </span>
                      <span
                        className={`inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-[11px] font-semibold leading-4 ${
                          item.status === 'completed'
                            ? 'bg-emerald-100 text-emerald-700'
                            : item.status === 'failed'
                              ? 'bg-red-100 text-red-700'
                              : item.status === 'canceled'
                                ? 'bg-gray-100 text-gray-500'
                                : 'bg-blue-100 text-blue-700'
                        }`}
                      >
                        {statusLabel(item)}
                        {item.error_message ? (
                          <Tooltip
                            arrow
                            describeChild
                            title={
                              <span className="whitespace-pre-wrap break-all">
                                {item.error_message}
                              </span>
                            }
                          >
                            <button
                              type="button"
                              aria-label={zh('查看错误详情', 'View error details')}
                              className="inline-flex h-4 w-4 items-center justify-center rounded-full"
                            >
                              <InfoOutlinedIcon sx={{ fontSize: 14 }} />
                            </button>
                          </Tooltip>
                        ) : null}
                      </span>
                      {item.network_optimized ? (
                        <span
                          className="inline-flex rounded-full bg-violet-100 px-1.5 py-0.5 text-[11px] font-semibold leading-4 text-violet-700"
                          title={zh(
                            '先在本机临时目录剪辑，再写回网络盘',
                            'Clip locally first, then write back to the network drive'
                          )}
                        >
                          {zh('NAS 加速', 'NAS optimized')}
                        </span>
                      ) : null}
                      <span className="text-[10px] font-medium text-gray-500">
                        {formatTime(item.created_at)}
                      </span>
                    </div>
                    <div
                      className="mt-0.5 truncate text-[11px] text-gray-400"
                      title={item.output_path}
                    >
                      {formatSeconds(item.start_seconds)} – {formatSeconds(item.end_seconds)} ·{' '}
                      {item.output_path}
                    </div>
                  </div>
                  <div className="flex shrink-0 gap-1.5">
                    {!containerMode && item.status === 'completed' ? (
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() => runAction(item, revealClipTask)}
                        aria-label={zh('打开剪辑所在位置', 'Reveal clip in folder')}
                        title={zh('打开所在位置', 'Reveal in folder')}
                        className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-gray-200 bg-white text-[16px] text-gray-600 hover:bg-gray-50 disabled:opacity-50"
                      >
                        <FolderOpenOutlinedIcon fontSize="inherit" />
                      </button>
                    ) : null}
                    {item.status === 'running' ? (
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() =>
                          runAction(
                            item,
                            cancelClipTask,
                            zh('已取消剪辑任务', 'Clip task canceled')
                          )
                        }
                        aria-label={zh('取消剪辑任务', 'Cancel clip task')}
                        title={zh('取消剪辑任务', 'Cancel clip task')}
                        className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-amber-200 bg-amber-50 text-[16px] text-amber-700 disabled:opacity-50"
                      >
                        <CancelOutlinedIcon fontSize="inherit" />
                      </button>
                    ) : null}
                    {terminal ? (
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() => runAction(item, deleteClipTask)}
                        aria-label={zh('移除剪辑记录', 'Remove clip task')}
                        title={zh('移除记录', 'Remove record')}
                        className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-gray-200 bg-white text-[16px] text-gray-600 hover:bg-gray-50 disabled:opacity-50"
                      >
                        <DeleteOutlineOutlinedIcon fontSize="inherit" />
                      </button>
                    ) : null}
                  </div>
                </div>
                <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100">
                  <div
                    className={`h-full rounded-full transition-all ${
                      item.status === 'failed'
                        ? 'bg-red-500'
                        : item.status === 'canceled'
                          ? 'bg-gray-400'
                          : item.status === 'completed'
                            ? 'bg-emerald-500'
                            : 'bg-blue-600'
                    }`}
                    style={{ width: `${progress}%` }}
                  />
                </div>
                <div className="mt-0.5 text-right text-[11px] text-gray-400">
                  {progress.toFixed(1)}%
                </div>
              </article>
            )
          })}
        </div>
      )}
    </div>
  )
}
