## 2026-07-25 - Semantic Pagination Components
**Learning:** Pagination components are frequently built with generic `<div>` containers, which screen readers fail to recognize as navigation elements. Additionally, visual spacers like ellipses create auditory noise.
**Action:** Always wrap pagination components in a `<nav aria-label="Pagination">`, add descriptive `aria-label` attributes to all navigation buttons (Previous, Next, individual pages), and hide purely visual spacers with `aria-hidden="true"`.
