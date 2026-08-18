## 2026-08-18 - Improve Pagination accessibility
**Learning:** Replaced non-semantic wrapper `div` with `<nav>` and added missing `aria-label`s on next/previous buttons to improve screen reader accessibility of the `Pagination` component. Using `aria-hidden="true"` on the ellipsis prevents confusion.
**Action:** When creating or maintaining navigation components, ensure semantic `<nav>` elements are used with proper `aria-label`s for the container and its interactive children.
