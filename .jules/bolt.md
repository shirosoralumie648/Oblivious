## 2024-06-14 - Cache Intl formatters
**Learning:** `Intl.NumberFormat` and `Intl.DateTimeFormat` instantiations are surprisingly expensive and create overhead on every render if called within functional components or mapping loops.
**Action:** Always declare `Intl` formatter instances as module-level constants and reuse them across calls.
