## 2026-10-27 - Referential equality in memoized default parameters
**Learning:** When optimizing React components with React.memo, default parameters using inline object instantiation (e.g., selectedIds = new Set<string>()) create a new reference on every render, defeating the memoization.
**Action:** Extract default object instantiations to a module-level constant (e.g., const EMPTY_SET = new Set<string>();) to maintain referential equality and preserve the performance benefits of memoization.
