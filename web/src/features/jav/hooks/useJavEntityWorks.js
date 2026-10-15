import { useCallback, useEffect, useMemo, useState } from 'react'

import { JAV_ENTITY_OPEN_MODE_MODAL, normalizeJavEntityOpenMode } from '@/constants/jav'

function normalizeEntity(type, item) {
  const id = Number(item?.id)
  if (!Number.isSafeInteger(id) || id <= 0) return null
  return {
    type,
    id,
    name: String(item?.name || item?.chinese_name || item?.japanese_name || '').trim(),
  }
}

export default function useJavEntityWorks({
  config,
  onSelectIdolPage,
  onSelectStudioPage,
  onSelectSeriesPage,
}) {
  const openMode = useMemo(
    () => normalizeJavEntityOpenMode(config?.jav_entity_open_mode),
    [config?.jav_entity_open_mode]
  )
  const [entityWorksModal, setEntityWorksModal] = useState(null)

  useEffect(() => {
    if (openMode !== JAV_ENTITY_OPEN_MODE_MODAL) setEntityWorksModal(null)
  }, [openMode])

  const selectEntity = useCallback(
    (type, item, pageHandler) => {
      const entity = normalizeEntity(type, item)
      if (!entity) return
      if (openMode === JAV_ENTITY_OPEN_MODE_MODAL) {
        setEntityWorksModal(entity)
        return
      }
      pageHandler?.(item)
    },
    [openMode]
  )

  const selectCatalogIdol = useCallback(
    (item) => selectEntity('idol', item, onSelectIdolPage),
    [onSelectIdolPage, selectEntity]
  )
  const selectCatalogStudio = useCallback(
    (item) => selectEntity('studio', item, onSelectStudioPage),
    [onSelectStudioPage, selectEntity]
  )
  const selectCatalogSeries = useCallback(
    (item) => selectEntity('series', item, onSelectSeriesPage),
    [onSelectSeriesPage, selectEntity]
  )
  const closeEntityWorks = useCallback(() => setEntityWorksModal(null), [])

  return {
    entityWorksModal,
    closeEntityWorks,
    selectCatalogIdol,
    selectCatalogStudio,
    selectCatalogSeries,
  }
}
