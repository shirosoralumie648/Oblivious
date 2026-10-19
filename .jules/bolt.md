## 2025-02-18 - Memoize Table Row Selections
**Learning:** React datatables that support row selection re-render frequently (e.g. on every selection change). Performing O(n) operations like `.map` and `.filter` over all rows directly in the component body can cause significant lag when the data set is large.
**Action:** Always memoize derived array computations (like `selectableRows` and `selectedCount`) in list or table components that accept large `data` arrays and respond to frequent state changes.
