## 2026-07-30 - Cache Intl Formatters
**Learning:** Instantiating `Intl.NumberFormat` or `Intl.DateTimeFormat` inside render functions or frequently called helpers causes rendering overhead.
**Action:** Always declare `Intl` formatter instances as module-level constants and reuse them across calls.
