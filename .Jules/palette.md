## 2026-07-14 - [Improve Pagination Accessibility]
**Learning:** Pagination components often lack semantic structure and context for screen readers. In this app's design system, standard React components mapping array iterations to buttons need explicit `aria-label`s and semantic `<nav>` wrappers to be fully accessible.
**Action:** Use semantic `<nav aria-label="Pagination">`, add `aria-label`s to Next/Previous buttons, add `aria-label`s like 'Go to page X' for numbered buttons, and use `aria-hidden="true"` on spacer ellipses.
