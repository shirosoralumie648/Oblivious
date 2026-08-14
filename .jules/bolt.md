## 2024-05-18 - Intl.NumberFormat and Intl.DateTimeFormat caching
**Learning:** Instantiating `Intl.NumberFormat` and `Intl.DateTimeFormat` on every render or function call is extremely expensive and causes overhead.
**Action:** Always declare `Intl.NumberFormat` and `Intl.DateTimeFormat` instances as module-level constants and reuse them across calls.