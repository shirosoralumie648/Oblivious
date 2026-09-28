## 2024-05-23 - DataTable Performance Overhead
**Learning:** The DataTable component was mapping and filtering `data` on every render to determine selected counts and selectable rows, leading to O(N) operations even when unrelated props changed.
**Action:** Always memoize derived array computations in heavily-used generic UI components like tables.
