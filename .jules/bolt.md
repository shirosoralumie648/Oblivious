
## 2026-08-02 - Optimize Intl object instantiation
**Learning:** Instantiating `Intl.NumberFormat` and `Intl.DateTimeFormat` on every function call or render loop introduces significant performance overhead and memory pressure.
**Action:** Always extract `Intl` formatters to module-level constants and reuse them across function calls.
