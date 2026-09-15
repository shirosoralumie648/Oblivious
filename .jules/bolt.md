## 2025-02-14 - Admin Page Filter Debouncing
**Learning:** Found a performance bottleneck where rapid typing in multiple text input filters on AdminUsageLogsPage triggers an API request for every single keystroke.
**Action:** Implemented a generic `useDebounce` hook and wrapped the filters state before passing to `loadUsageLogs` to prevent excessive API requests.
