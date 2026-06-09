## 2026-06-09 - DataTable Performance Bottlenecks
**Learning:** React re-renders in shared components like `DataTable` and `FilterPanel` recalculate arrays on every render (e.g. mapping `selectableRows` over the entire data set). In large tables or frequently rendering panels, this leads to unnecessary work on the main thread and jank.
**Action:** Use `useMemo` for derived collections and `useCallback` for event handlers passed downwards in highly-used components (like `DataTable` where data prop might be large).
