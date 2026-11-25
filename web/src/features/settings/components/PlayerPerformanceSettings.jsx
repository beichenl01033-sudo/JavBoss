import { zh } from '@/utils/i18n'

export default function PlayerPerformanceSettings({
  seekMode,
  hwdec,
  onSeekModeChange,
  onHWDecChange,
}) {
  return (
    <section className="space-y-3 border-t border-zinc-100 pt-4 dark:border-ui-border">
      <h3 className="text-sm font-semibold">{zh('跳转与解码', 'Seeking and decoding')}</h3>
      <label className="flex flex-wrap items-center justify-between gap-3 text-sm">
        <span>{zh('快进／后退方式', 'Seek mode')}</span>
        <select
          aria-label={zh('MPV 跳转方式', 'MPV seek mode')}
          className="form-field rounded-xl border px-3 py-2"
          value={seekMode}
          onChange={(event) => onSeekModeChange(event.target.value)}
        >
          <option value="adaptive">
            {zh('智能：单击精准，长按快速浏览', 'Adaptive: precise taps, fast holds')}
          </option>
          <option value="exact">{zh('始终精准（原有方式）', 'Always precise (legacy)')}</option>
        </select>
      </label>
      <p className="text-xs text-zinc-500 dark:text-ui-faint">
        {zh(
          '智能模式合并长按请求，按关键帧浏览，松开后精准定位。适用于本地和 NAS；不改变亮点跳转、剪辑起止点或视频文件。',
          'Adaptive mode coalesces held-key requests, previews keyframes, then seeks precisely on release. Works for local and NAS files without changing highlights, clip points or media files.'
        )}
      </p>
      <label className="flex flex-wrap items-center justify-between gap-3 text-sm">
        <span>{zh('视频解码', 'Video decoding')}</span>
        <select
          aria-label={zh('MPV 视频解码', 'MPV video decoding')}
          className="form-field rounded-xl border px-3 py-2"
          value={hwdec}
          onChange={(event) => onHWDecChange(event.target.value)}
        >
          <option value="no">{zh('软件解码（原有默认值）', 'Software (existing default)')}</option>
          <option value="auto">{zh('自动硬件解码', 'Automatic hardware decoding')}</option>
        </select>
      </label>
      <p className="text-xs text-zinc-500 dark:text-ui-faint">
        {zh(
          '自动模式会尝试受支持的硬解，不支持时回退软件解码，并不保证更快。可在 MPV 三个点菜单查看实际解码状态。保存会关闭当前 MPV 窗口，重新播放后生效。',
          'Automatic mode tries supported hardware decoding and falls back to software; it is not always faster. The MPV overflow menu shows the actual decoder. Saving closes the current MPV window; reopen a video to apply.'
        )}
      </p>
    </section>
  )
}
