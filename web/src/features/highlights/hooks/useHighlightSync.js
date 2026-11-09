import { useEffect } from 'react'
import { useStore } from '@/store'
import { fetchHighlightCounts } from '@/features/highlights/api'

export default function useHighlightSync() {
  useEffect(() => {
    let stopped = false
    let timer
    let signature = null
    let running = false
    const controller = new AbortController()
    const refresh = async () => {
      if (running || stopped) return
      running = true
      clearTimeout(timer)
      try {
        if (!document.hidden) {
          const counts = await fetchHighlightCounts({ signal: controller.signal })
          if (stopped) return
          const next = JSON.stringify(counts)
          if (signature === next) return
          const changed = signature !== null && signature !== next
          signature = next
          const patchVideo = (video) => ({
            ...video,
            highlight_count: Number(counts[video.id]) || 0,
          })
          useStore.setState((state) => ({
            videos: state.videos.map(patchVideo),
            javItems: state.javItems.map((item) => ({
              ...item,
              videos: (item.videos || []).map(patchVideo),
            })),
          }))
          const state = useStore.getState()
          if (changed && (state.highlightOnly || state.highlightExclude)) {
            if (state.viewMode === 'video') void state.loadVideos({ force: true, reconcile: true })
            else if (state.javTab === 'list') void state.loadJavs({ force: true, reconcile: true })
          }
          if (changed) window.dispatchEvent(new Event('javboss-highlights-changed'))
        }
      } catch (error) {
        if (error.name !== 'AbortError') console.warn('Refresh highlights failed', error)
      } finally {
        running = false
        if (!stopped) timer = setTimeout(refresh, 5000)
      }
    }
    void refresh()
    window.addEventListener('javboss-highlights-changed', refresh)
    return () => {
      stopped = true
      clearTimeout(timer)
      controller.abort()
      window.removeEventListener('javboss-highlights-changed', refresh)
    }
  }, [])
}
