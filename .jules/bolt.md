## 2026-09-16 - Prevent Unnecessary Data Table Processing

**Learning:** Recalculating selectable rows (via `data.map` and `filter`) on every render can cause measurable CPU overhead, especially when large administrative data sets are passed to the generic `DataTable` component.
**Action:** When working with shared data grids or tables rendering unbounded data arrays, aggressively memoize expensive operations like array transformations and filters that derive state from the primary `data` array prop.
