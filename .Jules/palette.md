## 2025-02-12 - Pagination Accessibility
**Learning:** Pagination components need semantic navigation wrappers and descriptive labels on all interactive elements. Visual spacers like ellipses should be hidden from screen readers to prevent confusion.
**Action:** Always use `<nav aria-label="Pagination">`, provide specific page context for each button label (e.g., `Go to page 5`), and hide non-interactive visual decorators.
