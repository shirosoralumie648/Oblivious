## 2026-07-29 - Reuse Intl Formatters
**Learning:** Instantiating Intl.NumberFormat and Intl.DateTimeFormat is expensive and creates unnecessary overhead on every render.
**Action:** Declare Intl instances as module-level constants and reuse them across calls instead of re-instantiating them inside components or frequently called utility functions.
