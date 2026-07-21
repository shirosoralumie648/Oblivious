## 2024-07-21 - Pagination Accessibility Nav Element
**Learning:** Pagination components often use `<div className="flex...">` wrappers. Using a semantic `<nav aria-label="Pagination">` instead immediately improves screen reader discoverability. Additionally, `aria-hidden="true"` is crucial for visual spacer elements like `...` to prevent confusing screen reader announcements.
**Action:** When implementing or modifying lists of links or buttons intended for navigation, always wrap them in a `<nav>` with a descriptive `aria-label`, and explicitly hide visual-only spacers.
