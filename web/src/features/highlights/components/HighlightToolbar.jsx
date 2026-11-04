import { useEffect, useRef, useState } from 'react'
import AppModal from '@/shared/ui/AppModal'
import { useStore } from '@/store'
import { videoQuery, javQuery } from '@/query/listQueries'
import { fetchVideos } from '@/features/video/api'
import { fetchJavs } from '@/features/jav/api'
import { playHighlights } from '@/features/highlights/api'
import { collectHighlightVideos } from '@/features/highlights/model'
import { getErrorMessage } from '@/utils/errors'
import { configFlag } from '@/utils/config'
import { generateRandomSeed } from '@/utils/urlState'
import { zh } from '@/utils/i18n'

export default function HighlightToolbar({ query, only, onOnlyChange }) {
  const highlightOnly = useStore((state) => state.highlightOnly)
  const enabled = useStore((state) => configFlag(state.config?.mpv_enabled, true))
  const [snapshot, setSnapshot] = useState(null)
  const [preview, setPreview] = useState(false)
  const [seconds, setSeconds] = useState(30)
  const [lead, setLead] = useState(0)
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const controller = useRef(null)
  useEffect(() => () => controller.current?.abort(), [])
  const close = () => {
    controller.current?.abort()
    setBusy(false)
    setSnapshot(null)
  }
  const open = () => {
    const state = useStore.getState()
    const jav = Boolean(query) || state.viewMode === 'jav'
    const params = query || (jav ? javQuery(state) : videoQuery(state))
    if (params.sort === 'random' && !params.seed) params.seed = generateRandomSeed()
    setSnapshot({ query: { ...params }, jav })
    setMessage('')
  }
  const start = async () => {
    const request = new AbortController()
    controller.current = request
    setBusy(true)
    setMessage('')
    try {
      const videos = await collectHighlightVideos({
        ...snapshot,
        fetcher: snapshot.jav ? fetchJavs : fetchVideos,
        signal: request.signal,
      })
      if (request.signal.aborted) return
      if (!videos.length) {
        setMessage(zh('当前筛选范围没有亮点。', 'No highlights match these filters.'))
        return
      }
      if (
        videos.length > 100 &&
        !window.confirm(
          zh(
            `将播放 ${videos.length} 个视频的亮点，是否继续？`,
            `Play highlights from ${videos.length} videos?`
          )
        )
      )
        return
      const result = await playHighlights(videos, {
        previewSeconds: preview ? seconds : 0,
        leadSeconds: lead,
        signal: request.signal,
      })
      if (!request.signal.aborted) {
        setMessage(
          zh(
            `已在 MPV 中打开 ${videos.length} 个视频、${result.count} 条亮点`,
            `Opened ${result.count} highlights from ${videos.length} videos in MPV`
          )
        )
        setSnapshot(null)
      }
    } catch (err) {
      if (err.name !== 'AbortError') setMessage(getErrorMessage(err))
    } finally {
      if (!request.signal.aborted) setBusy(false)
    }
  }
  return (
    <>
      <label className="filter-action-button cursor-pointer whitespace-nowrap">
        <input
          type="checkbox"
          checked={only ?? highlightOnly}
          onChange={(event) => {
            if (onOnlyChange) onOnlyChange(event.target.checked)
            else useStore.getState().setHighlightOnly(event.target.checked)
          }}
        />
        <span>{zh('仅看有亮点', 'With highlights')}</span>
      </label>
      <button
        type="button"
        disabled={!enabled}
        title={!enabled ? zh('当前模式不支持 MPV 播放', 'MPV playback is unavailable') : undefined}
        className="filter-action-button whitespace-nowrap disabled:opacity-40"
        onClick={open}
      >
        ★ {zh('播放亮点', 'Play highlights')}
      </button>
      {!snapshot && message && (
        <span role="status" className="text-xs text-slate-500">
          {message}
        </span>
      )}
      {snapshot && (
        <AppModal
          ariaLabel={zh('播放亮点', 'Play highlights')}
          onClose={close}
          zIndex={1500}
          contentClassName="w-[min(28rem,calc(100vw-2rem))] space-y-4 rounded-xl bg-white p-5 shadow-2xl"
        >
          <div className="flex items-center justify-between">
            <h2 className="font-semibold">
              ★ {zh('播放当前筛选范围的亮点', 'Play highlights matching current filters')}
            </h2>
            <button type="button" onClick={close} aria-label={zh('关闭', 'Close')}>
              ×
            </button>
          </div>
          <p className="text-sm text-slate-500">
            {zh(
              '包含所有分页。MPV 播放列表每行对应“视频 - 亮点”，点击任意一行直接播放。',
              'Includes every page. Each MPV playlist row is a video and highlight. Click any row to play it.'
            )}
          </p>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={preview}
              disabled={busy}
              onChange={(event) => setPreview(event.target.checked)}
            />
            {zh('自动逐个预览亮点', 'Automatically preview each highlight')}
          </label>
          {preview && (
            <label className="flex items-center justify-between gap-3 text-sm">
              {zh('每个亮点播放时长', 'Preview duration')}
              <select
                value={seconds}
                disabled={busy}
                onChange={(event) => setSeconds(Number(event.target.value))}
                className="rounded border p-2"
              >
                {[15, 30, 60, 120].map((value) => (
                  <option key={value} value={value}>
                    {value} {zh('秒', 'sec')}
                  </option>
                ))}
              </select>
            </label>
          )}
          <label className="flex items-center justify-between gap-3 text-sm">
            {zh('提前起播', 'Start before highlight')}
            <select
              value={lead}
              disabled={busy}
              onChange={(event) => setLead(Number(event.target.value))}
              className="rounded border p-2"
            >
              {[0, 3, 5, 10].map((value) => (
                <option key={value} value={value}>
                  {value} {zh('秒', 'sec')}
                </option>
              ))}
            </select>
          </label>
          {message && (
            <p role="alert" className="text-sm text-red-600">
              {message}
            </p>
          )}
          <button
            type="button"
            disabled={busy}
            onClick={start}
            className="w-full rounded-lg bg-sky-600 px-4 py-2 text-white disabled:opacity-50"
          >
            {busy
              ? zh('正在加载跨分页队列…', 'Loading all pages…')
              : zh('在 MPV 中播放', 'Play in MPV')}
          </button>
        </AppModal>
      )}
    </>
  )
}
