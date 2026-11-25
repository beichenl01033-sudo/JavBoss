import { useStore } from '@/store'
import { zh } from '@/utils/i18n'
import { normalizeLtxLocal, normalizeLtxCatalog } from '@/features/ltx/filters'

export default function LtxFilters({ local, catalog, onChange }) {
  const globalLocal = useStore((state) => state.ltxLocal)
  const globalCatalog = useStore((state) => state.ltxCatalog)
  const change = onChange || ((filters) => useStore.getState().setLtxFilters(filters))
  return (
    <>
      <label className="filter-action-button whitespace-nowrap">
        <span>{zh('本地 LTX', 'Local LTX')}</span>
        <select
          aria-label={zh('本地 LTX 筛选', 'Local LTX filter')}
          className="form-field bg-transparent"
          value={normalizeLtxLocal(local ?? globalLocal)}
          onChange={(event) => change({ ltxLocal: event.target.value })}
          title={zh(
            '视频按文件筛选；JAV 按当前目录内是否有 LTX 版筛选',
            'Files in videos; works with or without an LTX copy in the selected directory'
          )}
        >
          <option value="">{zh('全部', 'All')}</option>
          <option value="with">{zh('已有 LTX', 'With LTX')}</option>
          <option value="without">{zh('没有 LTX', 'Without LTX')}</option>
        </select>
      </label>
      <label className="filter-action-button whitespace-nowrap">
        <span>{zh('新站收录', 'LTX catalogue')}</span>
        <select
          aria-label={zh('LTX 新站收录筛选', 'LTX catalogue filter')}
          className="form-field bg-transparent"
          value={normalizeLtxCatalog(catalog ?? globalCatalog)}
          onChange={(event) => change({ ltxCatalog: event.target.value })}
          title={zh(
            '依据最近一次完整同步的目录；未关联番号的文件不归为未收录',
            'Uses the last complete catalogue; unlinked files are not classified as unlisted'
          )}
        >
          <option value="">{zh('全部', 'All')}</option>
          <option value="confirmed">{zh('已确认收录', 'Confirmed')}</option>
          <option value="pending">{zh('待确认', 'Pending')}</option>
          <option value="not_found">{zh('未收录', 'Not listed')}</option>
        </select>
      </label>
    </>
  )
}
