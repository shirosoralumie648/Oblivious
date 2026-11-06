## 2024-05-15 - Bolt's Journal Created\n**Learning:** Just starting out.\n**Action:** Let's find some performance wins.
## 2024-05-15 - Memoizing Generic React Components
**Learning:** When using React.memo to memoize a generic functional component (e.g., function DataTable<T>), the generic type inference is stripped. To preserve it, use a type assertion: export const DataTable = React.memo(DataTableInner) as typeof DataTableInner. Also, default object/Set parameters (like new Set()) must be extracted to module-level constants to preserve referential equality and prevent breaking memoization.
**Action:** Always extract default object/Set instantiations to module-level constants when using useMemo or React.memo, and use type assertions to preserve generic types.
