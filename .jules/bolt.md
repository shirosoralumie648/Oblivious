## 2024-10-06 - Combine Multiple Array Iterations
**Learning:** In React components rendering lists (like workflow executions), running multiple `.filter` and `.reduce` passes over the same nested array creates unnecessary CPU overhead and allocation.
**Action:** Consolidate multiple array methods into a single `for...of` or `.reduce` pass to minimize iterations.
