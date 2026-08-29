## 2024-05-14 - Pagination Accessibility
**Learning:** Pagination components often lack semantic structure (e.g., `<nav aria-label="Pagination">`) and specific `aria-label`s on individual buttons, making screen reader navigation confusing. Visual spacers like ellipses should also have `aria-hidden="true"`.
**Action:** Ensure all pagination components are wrapped in a semantic `<nav>` with an `aria-label`, hide visual spacers from screen readers, and provide explicit `aria-label`s on each navigational button.
