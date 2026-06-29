
## 2024-06-29 - [Skip unneeded iterations based on disabled props]
**Learning:** Found an O(n) array iteration (`filter` based on `map`) happening on every render in a generic `DataTable` component to calculate row selection state, even though the selection feature was disabled via the `selectable` prop. This resulted in unnecessary compute per render on potentially large collections.
**Action:** Always check if a potentially expensive data transformation is necessary based on feature flags/props before computing it, and wrap the minimal necessary logic in `useMemo` so it's only re-computed when inputs change.
