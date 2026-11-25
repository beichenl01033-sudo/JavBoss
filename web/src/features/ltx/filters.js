export const normalizeLtxLocal = (value) => (['with', 'without'].includes(value) ? value : '')
export const normalizeLtxCatalog = (value) =>
  ['confirmed', 'pending', 'not_found'].includes(value) ? value : ''

export const ltxQuery = (state) => ({
  ltxLocal: normalizeLtxLocal(state.ltxLocal),
  ltxCatalog: normalizeLtxCatalog(state.ltxCatalog),
})

export function appendLtxParams(params, filters) {
  const { ltxLocal, ltxCatalog } = ltxQuery(filters)
  if (ltxLocal) params.set('ltx_local', ltxLocal)
  if (ltxCatalog) params.set('ltx_catalog', ltxCatalog)
}

export function createLtxSlice({ set, lists }) {
  return {
    ltxLocal: '',
    ltxCatalog: '',
    setLtxFilters: (filters) =>
      set((state) => ({
        ...ltxQuery({ ...state, ...filters }),
        page: 1,
        javPage: 1,
        idolPage: 1,
        studioPage: 1,
        seriesPage: 1,
      })),
    refreshLtxResults: () => Promise.all(Object.values(lists).map((list) => list.reconcile())),
  }
}
