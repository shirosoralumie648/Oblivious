## 2025-07-09 - Reuse Intl Formatters to Avoid Instantiation Overhead
**Learning:** `Intl.NumberFormat` and `Intl.DateTimeFormat` are computationally expensive to instantiate. Repeatedly instantiating them inside render functions or utility formatters called frequently (like in React components such as `MetricCard.tsx`, `BillingPage.tsx`, and `MarketplaceMyAgentsPage.tsx`) creates unnecessary overhead and can cause performance bottlenecks.
**Action:** Always extract `Intl` formatters to module-level constants and reuse the same instance for formatting whenever the configuration is static.
