## 2023-10-24 - Skip Unused Selection Calculations in Shared Tables
**Learning:** Shared components like `DataTable` often perform expensive O(N) operations (e.g., mapping row IDs for selection state) on every render, even when those features (like `selectable=false`) are completely disabled for the current instance.
**Action:** Always wrap expensive list iterations in `useMemo` and include an early return or skip condition (like `if (!selectable)`) to prevent unnecessary work in shared UI components.
