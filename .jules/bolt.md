## 2024-06-25 - Memoizing derived arrays in list components
**Learning:** Generic list/table components that extract or derive properties (like row IDs) for every item in a large `data` array can suffer from performance degradation on every re-render (e.g. when checking a box updates state).
**Action:** Always wrap derived mapped data arrays (like extracting IDs from a generic dataset) in `useMemo` with dependencies on the dataset itself so re-renders from selection state don't re-trigger O(N) array transformations.
