
## 2026-10-31 - Memoizing generic React components with default parameters
**Learning:** When using React.memo on a generic functional component, the wrapper strips the generic type inference. Also, default parameters that instantiate objects/Sets inline (e.g., `selectedIds = new Set<string>()`) create new references every render, defeating memoization.
**Action:** Use a type assertion (e.g., `as typeof DataTableInner`) when exporting to preserve generics, and extract inline default objects to module-level constants (e.g., `const EMPTY_SET = new Set<string>();`).
