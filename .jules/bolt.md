## 2024-08-21 - [Optimize DataTable selectableRows]
**Learning:** Found a case in the `DataTable` component where an O(n) mapping (`data.map`) was recalculating `selectableRows` on every re-render (which includes selection state changes).
**Action:** Always wrap expensive list operations (like mapping/filtering large arrays) with `useMemo` in React when the result only depends on stable props (like `data` and `idKey`), especially in components that re-render frequently due to local state (like selection).
