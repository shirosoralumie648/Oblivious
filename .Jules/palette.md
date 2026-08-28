## 2026-08-28 - Pagination Screen Reader Accessibility
**Learning:** Found that generic wrapper divs and standard text buttons in pagination components fail to provide sufficient context for screen reader users navigating large lists. Semantic `<nav>` elements with `aria-label`s and hiding visual spacers with `aria-hidden` significantly improves the navigation experience.
**Action:** Always use `<nav aria-label="Pagination">`, provide descriptive `aria-label`s for all navigation buttons (e.g., 'Go to page X', 'Previous page'), and hide visual-only elements (like '...') from screen readers.
