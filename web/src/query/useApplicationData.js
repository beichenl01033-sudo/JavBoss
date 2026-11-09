import { useStore } from '@/store'
import { useShallow } from 'zustand/react/shallow'
import { useState, useMemo, useEffect } from 'react'
import useWatchedTimeSync from '@/query/useWatchedTimeSync'
import useHighlightSync from '@/features/highlights/hooks/useHighlightSync'

export default function useApplicationData() {
  useWatchedTimeSync()
  useHighlightSync()
  const {
    directories,
    selectedDirectoryId,
    videoSubdirectory,
    loadConfig,
    loadDirectories,
    loadTags,
    loadJavTags,
    videoHideJav,
  } = useStore(
    useShallow((state) => ({
      directories: state.directories,
      selectedDirectoryId: state.selectedDirectoryId,
      videoSubdirectory: state.videoSubdirectory,
      loadConfig: state.loadConfig,
      loadDirectories: state.loadDirectories,
      loadTags: state.loadTags,
      loadJavTags: state.loadJavTags,
      videoHideJav: state.videoHideJav,
    }))
  )
  const [configLoaded, setConfigLoaded] = useState(false)

  const directoryStateKey = useMemo(
    () =>
      [
        `selected:${selectedDirectoryId || 0}`,
        `subdirectory:${videoSubdirectory || ''}`,
        ...directories
          .filter((directory) => !directory?.is_delete)
          .map((directory) => `${directory.id}:${directory.enabled !== false ? '1' : '0'}`),
      ].join(','),
    [directories, selectedDirectoryId, videoSubdirectory]
  )

  useEffect(() => {
    let mounted = true
    loadConfig().finally(() => {
      if (mounted) setConfigLoaded(true)
    })
    return () => {
      mounted = false
    }
  }, [loadConfig])

  useEffect(() => {
    loadDirectories()
  }, [loadDirectories])

  useEffect(() => {
    loadTags({ skipUnchanged: true })
    loadJavTags({ skipUnchanged: true })
  }, [loadTags, loadJavTags, directoryStateKey, videoHideJav])
  return { configLoaded, directoryStateKey }
}
