import { useState } from 'react'
import HighlightModal from '@/features/highlights/components/HighlightModal'
import { highlightCount, uniqueHighlightVideos } from '@/features/highlights/model'
import { zh } from '@/utils/i18n'

export default function HighlightBadge({ videos, className = '' }) {
  const [openedVideos, setOpenedVideos] = useState(null)
  const count = highlightCount(videos)
  if (!count && !openedVideos) return null
  return (
    <>
      {count > 0 && (
        <button
          type="button"
          className={`rounded-full bg-amber-950/90 px-2 py-1 text-xs font-semibold text-amber-100 shadow hover:bg-amber-800 ${className}`}
          title={zh(`查看 ${count} 个亮点`, `View ${count} highlights`)}
          aria-label={zh(`查看 ${count} 个亮点`, `View ${count} highlights`)}
          onClick={(event) => {
            event.stopPropagation()
            setOpenedVideos(uniqueHighlightVideos(videos))
          }}
        >
          ★ {count}
        </button>
      )}
      {openedVideos && (
        <HighlightModal videos={openedVideos} onClose={() => setOpenedVideos(null)} />
      )}
    </>
  )
}
