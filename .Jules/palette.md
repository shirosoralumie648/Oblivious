## 2024-08-04 - Accessible Pagination
**Learning:** React generic pagination components frequently miss standard accessibility requirements like `aria-label` wrappers (`<nav aria-label="Pagination">`) and distinct labels for next/previous versus numeric page buttons. Wait-state visual gaps like "..." also need `aria-hidden="true"`.
**Action:** When building or auditing list controls, explicitly verify the presence of a semantic `<nav>` wrapper and distinct ARIA labels on all structural interactions, hiding visual-only spacers from screen readers.
