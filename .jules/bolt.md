## 2026-07-18 - Hoisting Intl Object Instantiations
**Learning:** `Intl.NumberFormat` and `Intl.DateTimeFormat` are expensive to instantiate. When they are created inside a render cycle or format function that runs frequently, it causes unnecessary performance overhead.
**Action:** Always extract `Intl.NumberFormat` and `Intl.DateTimeFormat` instantiations into module-level constants and reuse them across calls to improve performance.
