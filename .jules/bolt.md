## 2024-05-15 - Cache Intl formatters
**Learning:** Recreating `Intl.NumberFormat` and `Intl.DateTimeFormat` on every render or function call creates unnecessary overhead, as their instantiation is expensive. The codebase had this anti-pattern in multiple places.
**Action:** Always declare `Intl.NumberFormat` and `Intl.DateTimeFormat` instances as module-level constants and reuse them across calls to optimize performance.
