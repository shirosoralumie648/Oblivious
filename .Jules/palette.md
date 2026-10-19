## 2026-07-27 - [Accessibility] Improve screen reader support for Pagination
**Learning:** Pagination components lack semantic structure (`<nav>`) and descriptive ARIA labels, making them difficult for screen reader users to navigate. Ellipses used as visual spacers can also cause confusion if not hidden.
**Action:** Always use `<nav aria-label="Pagination">` for pagination containers, include descriptive `aria-label`s on previous/next and individual page buttons, and use `aria-hidden="true"` on visual spacers.
