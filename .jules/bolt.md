## 2024-05-18 - Module-level Intl Formatters
**Learning:** Instantiating `Intl.NumberFormat` and `Intl.DateTimeFormat` objects is an expensive operation that can cause unnecessary overhead on every render if placed inside a component body or formatting function. This is a specific codebase performance pattern for rendering optimization.
**Action:** Always declare `Intl.NumberFormat` and `Intl.DateTimeFormat` instances as module-level constants and reuse them across calls to minimize rendering and overhead costs.
