## 2024-09-26 - Skipping Expensive Computations in Shared Components
**Learning:** The shared `DataTable` component was doing O(N) array mappings (`data.map().filter()`) on every single render cycle (even for hover events) to calculate row selection state, even when `selectable` was set to `false`. This caused significant main thread blocking on large datasets.
**Action:** When building shared components with optional features (like table selection), conditionally bypass the expensive state calculations entirely if the feature flag/prop is disabled, and memoize the result.
