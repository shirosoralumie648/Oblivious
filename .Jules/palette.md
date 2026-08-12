## 2024-05-24 - Accessibility improvements for Pagination component
**Learning:** Adding standard ARIA roles (nav, aria-label, aria-hidden) to pagination controls, including ellipses spacers, drastically improves screen reader experiences for complex pagination navigation.
**Action:** Always wrap pagination components in a semantic `<nav aria-label="Pagination">`, assign clear `aria-label`s to navigation buttons, and ensure visual spacers like ellipses have `aria-hidden="true"`.
