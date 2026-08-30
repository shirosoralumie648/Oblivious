## 2024-05-18 - Avoid chained array methods in hot paths
**Learning:** Using chained array methods like `.map().filter().reduce()` in critical performance paths causes multiple array iterations and allocates intermediate arrays that must be garbage collected, creating noticeable slowdowns on large result sets.
**Action:** Replace chained array methods with a single `for...of` loop in hot paths for O(n) performance and zero intermediate allocations.
