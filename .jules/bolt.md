## 2024-05-14 - Expensive Intl.NumberFormat Instantiation
**Learning:** Re-instantiating `Intl.NumberFormat` on every render (or multiple times in utility functions called frequently) is a known performance bottleneck in React. It causes unnecessary overhead, especially in loops or frequently updated components.
**Action:** Extract `Intl.NumberFormat` and `Intl.DateTimeFormat` into module-level constants and reuse them across calls, as their instantiation is expensive and creates unnecessary overhead on every render.
