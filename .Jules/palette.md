## 2024-07-24 - Enhance Pagination Accessibility
**Learning:** Found that the Pagination component was missing proper semantic tags and aria-labels for screen readers. The '...' spacer was also reading out loud unnecessarily.
**Action:** Always wrap pagination components in `<nav aria-label="Pagination">`. Add `aria-label="Go to previous/next page"` to navigation buttons, `aria-label="Page N"` to page number buttons, and `aria-hidden="true"` to visual spacers like ellipses to reduce screen reader noise.
