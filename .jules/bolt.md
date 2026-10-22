## 2024-10-22 - Optimizing `DataTable` by memoizing selection and ID calculation
**Learning:** `DataTable` does multiple unmemoized loops (`data.map`) to calculate IDs and check `selectedIds` on every render.
**Action:** Use `useMemo` for derived states like `selectableRows`, `selectedCount`, `allSelected`, and `partiallySelected` to prevent unnecessary re-computations when data references haven't changed.
