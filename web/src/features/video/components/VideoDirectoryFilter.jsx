import FolderOpenOutlinedIcon from '@mui/icons-material/FolderOpenOutlined'
import { displayHostPath } from '@/utils/hostPath'
import { zh } from '@/utils/i18n'

export default function VideoDirectoryFilter({
  directories = [],
  selectedDirectoryId = null,
  onChange,
  useHostPaths = false,
}) {
  const availableDirectories = directories.filter(
    (directory) => directory && !directory.is_delete && directory.enabled !== false
  )

  return (
    <label className="flex min-w-0 items-center gap-2 text-sm text-gray-600">
      <FolderOpenOutlinedIcon className="shrink-0" fontSize="small" aria-hidden="true" />
      <span className="shrink-0">{zh('目录', 'Directory')}</span>
      <select
        value={selectedDirectoryId ?? ''}
        onChange={(event) => onChange?.(event.target.value || null)}
        aria-label={zh('按目录筛选视频', 'Filter videos by directory')}
        className="min-w-0 max-w-80 rounded-md border border-gray-300 bg-white px-2 py-1.5 text-sm text-gray-800 shadow-sm outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100 disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-400"
        disabled={availableDirectories.length === 0}
      >
        <option value="">{zh('全部目录', 'All directories')}</option>
        {availableDirectories.map((directory) => {
          const path = displayHostPath(directory.path, useHostPaths)
          return (
            <option key={directory.id} value={directory.id} title={path}>
              {path}
            </option>
          )
        })}
      </select>
    </label>
  )
}
