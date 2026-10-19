## 2024-05-24 - Reuse Intl formatters
**Learning:** Instantiating `Intl.NumberFormat` and `Intl.DateTimeFormat` within render functions or loops is an expensive operation and creates unnecessary overhead on every render in React.
**Action:** Always declare `Intl.NumberFormat` and `Intl.DateTimeFormat` instances as module-level constants and reuse them across calls, as their instantiation is expensive.
