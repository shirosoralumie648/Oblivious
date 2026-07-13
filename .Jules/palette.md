## 2026-07-13 - [Custom Pagination Accessibility]
**Learning:** Custom pagination components must use semantic `<nav aria-label="Pagination">` elements, include descriptive `aria-label`s on all navigation buttons, and use `aria-hidden="true"` on visual spacers (e.g., ellipses) to improve screen reader accessibility.
**Action:** When implementing custom pagination or reviewing existing ones, always check for `<nav>` tags, labels for numbered pages, and hiding purely visual dividers.
