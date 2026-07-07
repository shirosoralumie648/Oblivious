1. **Optimize `DataTable` in `src/web/src/components/shared/DataTable.tsx`**
   - Import `useMemo` from `react`.
   - Memoize the calculation of `selectableRows`, `selectedCount`, `allSelected`, and `partiallySelected` using `useMemo`. This prevents these calculations and array allocations from running on every re-render (which can happen frequently during typing or scrolling, especially when the table data changes).
   - The dependency array will include `data`, `idKey`, and `selectedIds`.

2. **Complete pre-commit steps to ensure proper testing, verification, review, and reflection are done.**
   - Call `pre_commit_instructions` and follow the provided steps.

3. **Submit the change.**
   - Submit the PR with the title: "⚡ Bolt: [performance improvement] Memoize selectable row state calculations in DataTable".
