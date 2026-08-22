## 2024-08-22 - Improved Pagination Accessibility
**Learning:** Pagination components that aren't wrapped in a semantic `<nav>` or lack descriptive `aria-label`s for navigation buttons are difficult to use with screen readers. Visual spacers like ellipses must also be explicitly hidden using `aria-hidden="true"`.
**Action:** When building or updating pagination components, ensure they use semantic `<nav aria-label="Pagination">` elements, include descriptive `aria-label`s on all navigation buttons, and use `aria-hidden="true"` on visual spacers.
