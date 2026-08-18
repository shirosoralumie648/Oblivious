## 2024-08-18 - Intl Instantiation Overhead
**Learning:** Instantiating `Intl.NumberFormat` and `Intl.DateTimeFormat` inside render functions or formatting helpers creates unnecessary overhead on every render, which is a specific anti-pattern in React apps displaying metrics.
**Action:** Always declare these `Intl` instances as module-level constants and reuse them.
