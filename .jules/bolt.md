## 2024-05-30 - Caching Intl Formatter Instances
**Learning:** `Intl.NumberFormat` and `Intl.DateTimeFormat` instantiations are surprisingly expensive and can cause performance overhead when created repeatedly during render cycles (e.g., inside formatting utility functions).
**Action:** Always declare `Intl` formatter instances as module-level constants and reuse them across calls rather than instantiating them dynamically per-call.
