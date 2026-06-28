## 2025-02-12 - Prevented expensive marketplace agent card re-renders
**Learning:** Rendering complex React components (like `AgentCard` with badges and star ratings) in a long list can significantly degrade performance during frequent state updates (like typing in a search bar), as React re-renders every item.
**Action:** Always wrap list item components in `memo()` when they are complex and driven by immutable data, especially if they are sibling components to input fields in a parent view.
