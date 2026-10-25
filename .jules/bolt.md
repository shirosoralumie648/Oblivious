## 2023-10-25 - React default parameter issue
**Learning:** Initializing objects like `new Set()` directly in functional component parameters causes referential instability. This means on every render, the default value evaluates to a *new* reference, defeating memoization patterns (like `React.memo` or `useMemo` dependencies).
**Action:** Always extract default objects/arrays/Sets to a module-level constant (e.g., `const EMPTY_SET = new Set<string>();`) instead of inline instantiation.
