import FolderOpenOutlinedIcon from '@mui/icons-material/FolderOpenOutlined'
import { displayHostPath } from '@/utils/hostPath'
import { zh } from '@/utils/i18n'

export default function DirectoryFilter({
  directories = [],
  selectedDirectoryId = null,
  onChange,
  useHostPaths = false,
}) {
  const availableDirectories = directories.filter(
    (directory) => directory && !directory.is_delete && directory.enabled !== false
  )

  return (
    <label className="directory-filter">
      <FolderOpenOutlinedIcon
        className="directory-filter__icon"
        fontSize="small"
        aria-hidden="true"
      />
      <select
        value={selectedDirectoryId ?? ''}
        onChange={(event) => onChange?.(event.target.value || null)}
        aria-label={zh('按目录筛选内容', 'Filter content by directory')}
        className="directory-filter__select"
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
