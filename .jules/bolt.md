## 2024-05-18 - Intl Formatter Optimization
**Learning:** Instantiating `Intl` formatters (`Intl.NumberFormat`, `Intl.DateTimeFormat`) inside rendering paths or frequently called functions is an expensive operation and creates unnecessary CPU overhead on every render.
**Action:** Always declare `Intl.NumberFormat` and `Intl.DateTimeFormat` instances as module-level constants and reuse them across calls.
