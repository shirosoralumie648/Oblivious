## 2024-05-24 - Extract Intl Formatters
**Learning:** `Intl.NumberFormat` and `Intl.DateTimeFormat` instantiations are expensive and create unnecessary overhead on every render if placed inline in React components.
**Action:** Always declare `Intl.NumberFormat` and `Intl.DateTimeFormat` instances as module-level constants and reuse them across calls to improve rendering performance.
