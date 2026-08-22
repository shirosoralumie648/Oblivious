## 2024-03-24 - Avoid unconditional iteration in reusable UI components
**Learning:** The DataTable component was computing derived selection state (`data.map` and `.filter`) unconditionally on every render, even for tables where `selectable` was false. This caused hidden O(N) overhead across the application for every re-render of every table.
**Action:** When computing derived array states (like mapped IDs or selected counts), wrap them in `useMemo` and always check the feature flag (e.g., `selectable`) first to completely short-circuit the iteration when the feature is disabled.
