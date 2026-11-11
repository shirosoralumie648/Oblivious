## 2024-11-11 - [React Default Parameter Performance]
**Learning:** In React components like DataTable, using `new Set<string>()` as a default parameter for props creates a new Set instance on every render when the prop is omitted. This breaks referential equality and can lead to unnecessary re-renders in child components or hooks that depend on that prop.
**Action:** Extract default object or Set instances to a module-level constant (e.g., `const EMPTY_SET = new Set<string>();`) to maintain referential equality across renders.
