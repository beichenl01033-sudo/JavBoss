import { useEffect, useState } from 'react'
import AppModal from '@/shared/ui/AppModal'
import { editHighlight, fetchHighlights, playHighlights } from '@/features/highlights/api'
import { formatHighlightTime } from '@/features/highlights/model'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

function HighlightRow({ item, busy, onPlay, onEdit }) {
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [label, setLabel] = useState(item.label)
  return (
    <div className="flex flex-wrap items-center gap-2 rounded-lg bg-slate-50 px-3 py-2 dark:bg-ui-canvas">
      <button
        type="button"
        disabled={busy}
        onClick={onPlay}
        className="font-mono text-sm font-semibold text-sky-700 hover:underline dark:text-sky-300"
        aria-label={zh(
          `播放亮点 ${formatHighlightTime(item.time)}`,
          `Play highlight ${formatHighlightTime(item.time)}`
        )}
      >
        ▶ {formatHighlightTime(item.time)}
      </button>
      {editing ? (
        <form
          className="flex min-w-0 flex-1 gap-2"
          onSubmit={async (event) => {
            event.preventDefault()
            if (await onEdit(label)) setEditing(false)
          }}
        >
          <input
            aria-label={zh('亮点名称', 'Highlight name')}
            maxLength={200}
            required
            value={label}
            onChange={(event) => setLabel(event.target.value)}
            className="form-field min-w-0 flex-1 rounded border px-2 py-1 text-sm"
          />
          <button
            disabled={busy || !label.trim()}
            className="text-sm text-sky-700 dark:text-sky-300"
          >
            {zh('保存', 'Save')}
          </button>
          <button
            type="button"
            onClick={() => setEditing(false)}
            className="text-sm text-slate-500 dark:text-ui-faint"
          >
            {zh('取消', 'Cancel')}
          </button>
        </form>
      ) : (
        <span className="min-w-0 flex-1 break-words text-sm">
          {item.label || zh('亮点', 'Highlight')}
        </span>
      )}
      {!editing && (
        <button
          type="button"
          disabled={busy}
          onClick={() => {
            setLabel(item.label)
            setEditing(true)
            setDeleting(false)
          }}
          className="text-xs text-slate-500 hover:text-sky-700 dark:text-ui-faint dark:hover:text-sky-300"
        >
          {zh('重命名', 'Rename')}
        </button>
      )}
      <button
        type="button"
        disabled={busy}
        onClick={async () => {
          if (!deleting) setDeleting(true)
          else if (await onEdit(null)) setDeleting(false)
        }}
        className="text-xs text-red-600 dark:text-red-300"
      >
        {deleting ? zh('确认删除标记', 'Confirm remove mark') : zh('删除', 'Delete')}
      </button>
      {deleting && (
        <button
          type="button"
          onClick={() => setDeleting(false)}
          className="text-xs text-slate-500 dark:text-ui-faint"
        >
          {zh('取消', 'Cancel')}
        </button>
      )}
    </div>
  )
}

export default function HighlightModal({ videos, onClose }) {
  const [groups, setGroups] = useState([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    const controller = new AbortController()
    const load = () =>
      Promise.all(
        videos.map(async (video) => ({
          video,
          doc: await fetchHighlights(video.id, { signal: controller.signal }),
        }))
      )
        .then((items) => {
          if (!controller.signal.aborted) setGroups(items)
        })
        .catch((err) => {
          if (err.name !== 'AbortError') setError(getErrorMessage(err))
        })
        .finally(() => {
          if (!controller.signal.aborted) setLoading(false)
        })
    void load()
    window.addEventListener('javboss-highlights-changed', load)
    return () => {
      controller.abort()
      window.removeEventListener('javboss-highlights-changed', load)
    }
  }, [videos])

  const perform = async (action) => {
    setBusy(true)
    setError('')
    try {
      await action()
      return true
    } catch (err) {
      setError(getErrorMessage(err))
      return false
    } finally {
      setBusy(false)
    }
  }
  const edit = (group, item, label) =>
    perform(async () => {
      try {
        const doc = await editHighlight(group.video.id, {
          time: item.time,
          label,
          revision: group.doc.revision,
        })
        setGroups((current) =>
          current.map((entry) => (entry.video.id === group.video.id ? { ...entry, doc } : entry))
        )
        window.dispatchEvent(new Event('javboss-highlights-changed'))
      } catch (err) {
        const doc = await fetchHighlights(group.video.id)
        setGroups((current) =>
          current.map((entry) => (entry.video.id === group.video.id ? { ...entry, doc } : entry))
        )
        throw err
      }
    })

  return (
    <AppModal
      ariaLabel={zh('亮点', 'Highlights')}
      onClose={onClose}
      zIndex={1500}
      contentClassName="flex max-h-[85vh] w-[min(42rem,calc(100vw-2rem))] flex-col rounded-xl bg-white dark:bg-ui-surface shadow-2xl"
    >
      <header className="flex items-center justify-between border-b px-5 py-4">
        <div>
          <h2 className="font-semibold">★ {zh('亮点', 'Highlights')}</h2>
          <p className="mt-1 text-xs text-slate-500 dark:text-ui-faint">
            {zh(
              '点击时间直接播放；删除仅移除标记，不删除视频。',
              'Click a time to play. Removing a mark does not delete the video.'
            )}
          </p>
        </div>
        <button
          type="button"
          onClick={onClose}
          className="p-2 text-xl"
          aria-label={zh('关闭亮点', 'Close highlights')}
        >
          ×
        </button>
      </header>
      <div className="space-y-5 overflow-y-auto p-5">
        {error && (
          <p role="alert" className="text-sm text-red-600 dark:text-red-300">
            {error}
          </p>
        )}
        {loading && <p role="status">{zh('加载中…', 'Loading…')}</p>}
        {groups.map((group) => (
          <section key={group.video.id}>
            <h3 className="mb-2 break-all text-sm font-semibold">
              {group.video.filename || `Video #${group.video.id}`}
            </h3>
            <div className="space-y-2">
              {!group.doc.items.length && (
                <p className="text-sm text-slate-500 dark:text-ui-faint">
                  {zh('暂无亮点', 'No highlights')}
                </p>
              )}
              {group.doc.items.map((item, index) => (
                <HighlightRow
                  key={`${item.time}-${index}`}
                  item={item}
                  busy={busy}
                  onPlay={() =>
                    perform(() => playHighlights([{ ...group.video, highlight_time: item.time }]))
                  }
                  onEdit={(label) => edit(group, item, label)}
                />
              ))}
            </div>
          </section>
        ))}
      </div>
    </AppModal>
  )
}
