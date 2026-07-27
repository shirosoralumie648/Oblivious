## 2026-07-27 - [Extract Intl formatters to module scope]
**Learning:** Instantiating `Intl.NumberFormat` and `Intl.DateTimeFormat` inside render cycles or frequently called utility functions causes unnecessary overhead.
**Action:** Always declare `Intl` formatter instances as module-level constants and reuse them across calls.
