## 2025-02-18 - Hoist Intl formatters outside render path
**Learning:** Instantiating `Intl.NumberFormat` and `Intl.DateTimeFormat` objects inside React components (or frequently called render utilities) is an expensive operation in JavaScript that creates measurable overhead on every render cycle.
**Action:** Always declare `Intl.NumberFormat` and `Intl.DateTimeFormat` instances as module-level constants and reuse them across format calls rather than creating a new instance every time a value needs to be formatted.
