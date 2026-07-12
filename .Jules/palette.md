## 2025-02-20 - [Accessibility] Use semantic `<nav>` for Pagination components

**Learning:** When creating pagination controls, grouping them within a simple `<div>` misses out on accessibility benefits. Screen readers treat it like normal layout structure.

**Action:** Always wrap pagination components in a `<nav aria-label="Pagination">` element. This creates a distinct landmark for screen reader users. Additionally, add explicit `aria-label` to "Previous"/"Next" and individual page buttons, and hide visual spacers like ellipses `...` from screen readers using `aria-hidden="true"`.
