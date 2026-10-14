import {
  fetchDirectories,
  createDirectory,
  updateDirectory,
  deleteDirectory as deleteDirectoryApi,
} from '@/features/directories/api'
import { zh } from '@/utils/i18n'
import { directoryScopeResetState } from '@/state/model'

export function createDirectorySlice({ set, get, invalidateDirectoryScopedRequests }) {
  return {
    directories: [],
    loadDirectories: async () => {
      try {
        const directories = await fetchDirectories()
        const active = directories.filter((d) => !d.is_delete)
        const selectedDirectoryId = Number(get().selectedDirectoryId)
        const selectionAvailable = active.some(
          (directory) => directory.enabled !== false && Number(directory.id) === selectedDirectoryId
        )
        set({
          directories: active,
          selectedDirectoryId: selectionAvailable ? selectedDirectoryId : null,
        })
      } catch (e) {
        console.error(zh('加载目录失败', 'Failed to load directories'), e)
      }
    },
    createDirectory: async ({ path }) => {
      const dir = await createDirectory({ path })
      const next = dir && !dir.is_delete ? [...get().directories, dir] : get().directories
      invalidateDirectoryScopedRequests()
      set({ directories: next, ...directoryScopeResetState() })
      return dir
    },
    updateDirectory: async (id, payload) => {
      const dir = await updateDirectory(id, payload)
      const state = get()
      const next = state.directories
        .map((d) =>
          d.id === id
            ? {
                ...d,
                ...dir,
                scanned_video_count: d.scanned_video_count,
                scraped_video_count: d.scraped_video_count,
                is_scanning: d.is_scanning,
                work_status: d.work_status,
              }
            : d
        )
        .filter((d) => d && !d.is_delete)
      const scopeChanged =
        Object.prototype.hasOwnProperty.call(payload || {}, 'enabled') || Boolean(dir?.is_delete)
      if (scopeChanged) invalidateDirectoryScopedRequests()
      const selectionAvailable = next.some(
        (directory) =>
          directory.enabled !== false && Number(directory.id) === Number(state.selectedDirectoryId)
      )
      set({
        directories: next,
        selectedDirectoryId: selectionAvailable ? state.selectedDirectoryId : null,
        ...(scopeChanged ? directoryScopeResetState() : {}),
      })
      return dir
    },
    deleteDirectory: async (id) => {
      const dir = await deleteDirectoryApi(id)
      const state = get()
      const next = state.directories
        .map((d) => (d.id === id ? dir : d))
        .filter((d) => d && !d.is_delete)
      invalidateDirectoryScopedRequests()
      set({
        directories: next,
        selectedDirectoryId:
          Number(state.selectedDirectoryId) === Number(id) ? null : state.selectedDirectoryId,
        ...directoryScopeResetState(),
      })
      return dir
    },
  }
}
