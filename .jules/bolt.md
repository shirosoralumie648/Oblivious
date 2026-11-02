## 2024-11-02 - Memoization of Generic React Components
**Learning:** Default object parameters in React components (like `new Set()`) defeat memoization because they create a new reference on every render. Additionally, wrapping a generic component in `React.memo` drops its generic type inference.
**Action:** Extract default object instantiations to module-level constants (e.g., `const EMPTY_SET = new Set()`) to maintain referential equality, and use type assertions (`as typeof ComponentInner`) when exporting memoized generic components to preserve their generic signature.
