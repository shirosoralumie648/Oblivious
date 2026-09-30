## 2024-05-24 - Unnecessary Array Iterations in DataTable
**Learning:** Reusable components like DataTable often perform O(N) operations (e.g., mapping data to row IDs) on every render. This can become a severe bottleneck when rendering large datasets or when parent components trigger frequent re-renders.
**Action:** Always memoize derived array computations (like selectable row IDs) in generic table or list components using `useMemo` to ensure they only recalculate when the underlying data or identity key changes.
