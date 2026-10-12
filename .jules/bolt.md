
## 2024-06-12 - Prevented unnecessary map/filter on default DataTable render
**Learning:** Default properties can lead to hidden performance costs. DataTable was mapping/filtering all rows to calculate selection state on every render, even though `selectable` defaults to `false`.
**Action:** When computing derived state that depends on an optional feature (like `selectable`), use an early return or condition to avoid O(N) operations when the feature is disabled.
