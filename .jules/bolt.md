## 2024-05-15 - [Avoid O(2n) filter().filter() in useMemo]
**Learning:** React components often combine `Array.prototype.filter()` calls sequentially within `useMemo` blocks, creating an O(2n) iteration anti-pattern. This is especially prevalent when combining a state-based filter (e.g. tabs/categories) with a text search filter.
**Action:** Always collapse sequential `.filter().filter()` operations into a single `.filter()` pass with compound conditions. This halves the iteration overhead and reduces memory allocation from intermediate arrays.
