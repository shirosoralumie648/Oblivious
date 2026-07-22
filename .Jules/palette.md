
## 2024-07-22 - Add semantic navigation to custom pagination
**Learning:** Custom pagination components often miss semantic `<nav>` elements, `aria-label`s, and `aria-hidden` attributes for visual spacers like ellipses, severely hindering screen reader accessibility.
**Action:** Always wrap pagination controls in `<nav aria-label="Pagination">`, label all interactive navigation elements, and hide visual spacers using `aria-hidden="true"`.
