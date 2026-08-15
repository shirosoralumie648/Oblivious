## 2026-08-15 - Cached Intl formatters for performance
**Learning:** Instantiating `Intl.NumberFormat` and `Intl.DateTimeFormat` on every render in React is an expensive operation that can cause performance degradation, particularly inside commonly re-rendered components (like `MetricCard`) or within tight lists (like `MarketplaceMyAgentsPage`).
**Action:** Extract `Intl` formatters to module-level constants to ensure they are instantiated once and reused across renders.
