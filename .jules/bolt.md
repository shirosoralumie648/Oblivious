## 2024-03-24 - Memoize derived state in DataTable
**Learning:** `DataTable` was computing `selectableRows`, `selectedCount`, `allSelected`, and `partiallySelected` on every single render by iterating over the `data` array (`O(N)`). For large tables or when multiple rows are rapidly selected/unselected, this blocks the main thread.
**Action:** Use `useMemo` to memoize the calculation of these variables.
