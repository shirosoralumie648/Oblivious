## 2024-10-24 - React Default Parameter Instantiation Breaks Memoization
**Learning:** Using inline object or Set instantiation (like `selectedIds = new Set<string>()`) for React component default parameters creates a new reference on every render. If the component is memoized, or if these parameters are used in hook dependencies, it defeats the optimization.
**Action:** Always extract default objects/Sets to a module-level constant (e.g., `const EMPTY_SET = new Set<string>()`) to maintain referential equality and preserve memoization benefits.
