import CloseRoundedIcon from '@mui/icons-material/CloseRounded'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useShallow } from 'zustand/react/shallow'

import { resolveJavSort } from '@/constants/jav'
import { fetchJavs } from '@/features/jav/api'
import JavGrid from '@/features/jav/components/JavGrid'
import Pagination from '@/shared/ui/Pagination'
import AppModal from '@/shared/ui/AppModal'
import { useStore } from '@/store'
import { getErrorMessage } from '@/utils/errors'
import { zh } from '@/utils/i18n'

const emptySelection = new Set()

const entityLabels = {
  idol: ['女优', 'Idol'],
  studio: ['片商', 'Studio'],
  series: ['系列', 'Series'],
}

export function javEntityFilters(entity) {
  const id = Number(entity?.id)
  const type = entity?.type
  return {
    idolIds: type === 'idol' && id > 0 ? [id] : [],
    studioId: type === 'studio' && id > 0 ? id : null,
    seriesId: type === 'series' && id > 0 ? id : null,
  }
}

export default function JavEntityWorksModal({ entity, onClose, buildJavUrl, actions }) {
  const {
    javPageSize,
    javGridColumns,
    javTitleMaxRows,
    javIdolTagMaxRows,
    javTagMaxRows,
    javSort,
    javSortRules,
    selectedDirectoryId,
  } = useStore(
    useShallow((state) => ({
      javPageSize: state.javPageSize,
      javGridColumns: state.javGridColumns,
      javTitleMaxRows: state.javTitleMaxRows,
      javIdolTagMaxRows: state.javIdolTagMaxRows,
      javTagMaxRows: state.javTagMaxRows,
      javSort: state.javSort,
      javSortRules: state.javSortRules,
      selectedDirectoryId: state.selectedDirectoryId,
    }))
  )
  const [page, setPage] = useState(1)
  const [items, setItems] = useState([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const scrollRef = useRef(null)
  const filters = useMemo(() => javEntityFilters(entity), [entity])
  const sort = useMemo(
    () =>
      resolveJavSort({
        javSort,
        javSortRules,
        javIdolIds: filters.idolIds,
        javStudioId: filters.studioId,
        javSeriesId: filters.seriesId,
      }).sort,
    [filters, javSort, javSortRules]
  )
  const pageSize = Math.max(1, Number(javPageSize) || 24)
  const lastPage = Math.max(1, Math.ceil(total / pageSize))

  useEffect(() => {
    const controller = new AbortController()
    setLoading(true)
    setError('')
    fetchJavs({
      limit: pageSize,
      offset: (page - 1) * pageSize,
      sort,
      directoryId: selectedDirectoryId,
      signal: controller.signal,
      ...filters,
    })
      .then((result) => {
        setItems(Array.isArray(result?.items) ? result.items : [])
        setTotal(Number(result?.total) || 0)
      })
      .catch((err) => {
        if (err?.name !== 'AbortError') setError(getErrorMessage(err))
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false)
      })
    return () => controller.abort()
  }, [filters, page, pageSize, selectedDirectoryId, sort])

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: 0 })
  }, [page])

  const typeLabel = entityLabels[entity?.type] || ['', '']
  const name = String(entity?.name || '').trim() || zh(typeLabel[0], typeLabel[1])
  const title = zh(`${name}的作品`, `${name}'s works`)

  return (
    <AppModal
      ariaLabel={title}
      className="p-4"
      contentClassName="flex h-[92vh] w-[min(96rem,calc(100vw-2rem))] flex-col overflow-hidden rounded-xl border border-gray-200 bg-white shadow-2xl"
      onClose={onClose}
      zIndex={1250}
    >
      <header className="flex shrink-0 items-center justify-between gap-4 border-b border-gray-100 px-5 py-3">
        <div className="min-w-0">
          <h2 className="truncate text-lg font-semibold text-gray-900">{title}</h2>
          <p className="mt-0.5 text-xs text-gray-500">
            {loading ? zh('加载中…', 'Loading…') : zh(`共 ${total} 部`, `${total} works`)}
          </p>
        </div>
        <button
          type="button"
          onClick={onClose}
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-gray-500 hover:bg-gray-100 hover:text-gray-900"
          aria-label={zh('关闭相关作品浮层', 'Close related works overlay')}
        >
          <CloseRoundedIcon sx={{ fontSize: 22 }} />
        </button>
      </header>

      <div ref={scrollRef} className="min-h-0 flex-1 overflow-y-auto p-5">
        <div className="mb-4 flex justify-center">
          <Pagination
            page={page}
            lastPage={lastPage}
            totalItems={total}
            hasPrev={page > 1}
            hasNext={page < lastPage}
            onFirst={() => setPage(1)}
            onPrev={() => setPage((current) => Math.max(1, current - 1))}
            onGoToPage={setPage}
            onNext={() => setPage((current) => Math.min(lastPage, current + 1))}
            onLast={() => setPage(lastPage)}
          />
        </div>
        {error ? (
          <div role="alert" className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-700">
            {error}
          </div>
        ) : loading ? (
          <div className="flex min-h-64 items-center justify-center rounded-lg border border-dashed border-gray-200 text-gray-500">
            {zh('加载中…', 'Loading…')}
          </div>
        ) : (
          <JavGrid
            items={items}
            selectedIds={emptySelection}
            selectionDisabled
            columns={javGridColumns}
            titleMaxRows={javTitleMaxRows}
            idolTagMaxRows={javIdolTagMaxRows}
            tagMaxRows={javTagMaxRows}
            buildJavUrl={buildJavUrl}
            {...actions}
          />
        )}
      </div>
    </AppModal>
  )
}
