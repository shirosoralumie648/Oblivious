## 2024-06-25 - Avoid inline object/Set instantiation for default parameters
**Learning:** Using `selectedIds = new Set<string>()` as a default parameter in a React component creates a new reference on every render, defeating memoization and triggering unnecessary downstream re-renders.
**Action:** Extract default objects, arrays, or Sets to a module-level constant (e.g., `const EMPTY_SET = new Set<string>();`) to maintain referential equality across renders.
