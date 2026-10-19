## 2024-05-18 - Caching Intl Formatters
**Learning:** Instantiating `Intl.NumberFormat` and `Intl.DateTimeFormat` objects can be surprisingly expensive in JavaScript. Doing this repeatedly inside component render cycles or loops can degrade frontend performance.
**Action:** Always extract `Intl` formatters into module-level constants (e.g., `const currencyFormatter = new Intl.NumberFormat(...)`) when the locale and formatting options are static, rather than instantiating them on every function call or render.
