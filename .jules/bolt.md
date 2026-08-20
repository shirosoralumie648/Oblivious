## 2025-02-23 - Prevent Intl formatter instantiation overhead
**Learning:** `Intl.NumberFormat` and `Intl.DateTimeFormat` are expensive to instantiate and can cause CPU spikes when created repeatedly during list/grid rendering.
**Action:** Always declare these formatters as module-level constants and reuse the instances when applying formatting in React components.
