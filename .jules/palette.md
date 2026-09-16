## 2026-09-16 - Add confirmation dialog to destructive UI actions
**Learning:** Destructive actions like deleting an MCP server were previously immediate, causing potential accidental deletions.
**Action:** Always add a confirmation step (e.g., `window.confirm`) to destructive actions such as deletions to improve safety and accessibility.
