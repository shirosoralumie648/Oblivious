## 2024-08-10 - Accessible Pagination Semantics
**Learning:** Pagination components using custom div wrappers and generic buttons lack sufficient context for screen readers to navigate page states intuitively, which is a recurring pattern in custom-built design systems.
**Action:** Always wrap pagination components in a `<nav aria-label="Pagination">` and ensure all navigation and discrete page buttons have explicit `aria-label`s (e.g., "Go to page X", "Go to previous page") and visual spacers use `aria-hidden="true"`.
