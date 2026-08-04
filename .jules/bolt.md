## 2024-05-24 - Expensive Intl Instantiations in Render Paths
**Learning:** Instantiating `Intl.NumberFormat` and `Intl.DateTimeFormat` inside React render functions or utility functions called during renders (like table cell formatters) creates significant unnecessary CPU overhead and GC pressure.
**Action:** Always declare `Intl` formatter instances as module-level constants and reuse them across calls.
