## 2024-09-18 - Avoid map().filter() chaining in render
**Learning:** Found a performance bottleneck in React render where `.map().filter()` chains caused unnecessary array allocations and O(N) operations on every render.
**Action:** Consolidate data transformation loops into a single `for` loop, and wrap the calculation in `useMemo` so it only recalculates when dependencies change.
