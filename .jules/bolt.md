## 2024-05-18 - Optimize Intl Formatters
**Learning:** Instantiating Intl formatters (e.g., Intl.NumberFormat) is expensive and causes unnecessary overhead on every render in React.
**Action:** Always declare Intl formatters as module-level constants and reuse them across function calls.
