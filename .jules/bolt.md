## 2026-08-11 - Expensive Intl.*Format Instantiations
**Learning:** Instantiating `Intl.NumberFormat` or `Intl.DateTimeFormat` inside render loops or frequently called helper functions is computationally expensive and causes unnecessary overhead.
**Action:** Always declare these formatters as module-level constants and reuse them across calls to improve rendering performance.
