## 2024-06-08 - Use Nav Landmarks and Aria-Labels for Pagination
**Learning:** Found that custom pagination controls built with divs were not announced as navigation landmarks to screen readers, making them harder to find and interact with. Additionally, numbered buttons need clear labels (like "Page 1" instead of just "1").
**Action:** Always wrap pagination controls in a `<nav aria-label="Pagination">` and add explicit `aria-label` properties to numeric page buttons to describe their purpose.
