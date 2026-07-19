## 2024-05-24 - Pagination Accessibility
**Learning:** Pagination components lack semantic `<nav>` wrappers and explicit ARIA labels on navigation elements.
**Action:** Always wrap pagination in `<nav aria-label="Pagination">`, use `aria-label` for buttons, and `aria-hidden="true"` on spacers.
