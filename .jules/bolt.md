## 2024-06-23 - Memoize DataTable row selection calculations
**Learning:** In heavily re-rendered list components like `DataTable.tsx`, deriving array computations (like mapping row IDs and filtering for selection checks) without memoization can cause UI lag because they execute on every render (including every row selection click).
**Action:** Use `useMemo` for derived array/collection computations tied to frequently updating states like selection sets, particularly if they iterate over the entire `data` prop.
