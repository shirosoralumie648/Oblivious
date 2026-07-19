## 2024-05-24 - Optimize Intl API instantiation
**Learning:** Recreating `Intl.NumberFormat` and `Intl.DateTimeFormat` on every function call or React render cycle is expensive and unnecessary.
**Action:** Always instantiate `Intl` formatter objects as module-level constants and reuse their `.format()` methods to minimize overhead.
