
## 2026-10-04 - Add loading states for async operations
**Learning:** Using conditional rendering with `RiLoader4Line` alongside `animate-spin` is a solid and clean way to indicate loading state on action buttons, especially when leveraging a `loadingAction` generic variable rather than many individual boolean loading flags in components like `McpServersPanel.tsx`.
**Action:** When working on complex panels with multiple distinct async actions per item, try storing a `loadingAction` string (e.g., `action:id`) and conditionally render `RiLoader4Line` where appropriate instead of creating an overly complex state machine.
