
## 2024-05-24 - Accessible Pagination Pattern
**Learning:** Using a simple `div` for pagination and unlabeled page buttons provides poor context for screen readers. Using a `<nav aria-label="Pagination">` and explicitly labeling page buttons like `aria-label="Page 1"` significantly improves the experience.
**Action:** Always wrap pagination components in `<nav aria-label="Pagination">` and ensure all interactive controls inside (like Previous/Next or specific page numbers) have explicit `aria-label`s.
