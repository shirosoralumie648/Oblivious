## 2025-02-14 - Extracting Intl instances to module scope
**Learning:** Instantiating `Intl.NumberFormat` and `Intl.DateTimeFormat` within render functions or inside mapping callbacks can cause noticeable CPU overhead, especially when frequent component renders occur (like MetricCards on dashboards) or when mapping large arrays.
**Action:** Always extract `Intl` formatters into module-scoped constants if they do not depend on dynamic locale strings, preventing unnecessary recreation on every render and improving performance.
