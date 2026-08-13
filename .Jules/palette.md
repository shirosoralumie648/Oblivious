## 2026-08-13 - Pagination Accessibility
**Learning:** Pagination components are often built with unsemantic `<div>`s and lack necessary screen reader context (like ARIA labels for buttons and hiding visual spacers).
**Action:** Use semantic `<nav aria-label="Pagination">`, include `aria-label` on all navigation buttons, and use `aria-hidden="true"` on visual spacers (e.g., ellipses).
