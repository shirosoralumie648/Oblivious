## 2026-11-10 - React.memo with inline Sets
**Learning:** Default parameters that initialize new objects (like `new Set<string>()`) defeat memoization because they create a new reference on every render when the prop is omitted.
**Action:** Always extract default object/collection instances to module-level constants (e.g., `const EMPTY_SET = new Set<string>();`) when optimizing React components with `useMemo` or `React.memo`.
