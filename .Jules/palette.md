## 2024-07-03 - Standardizing Loading Feedback on Async Buttons
**Learning:** Async operations lacking immediate visual feedback leave users uncertain if their click registered, especially for dynamic lists like MCP servers.
**Action:** Standardizing loading feedback by replacing the action icon with a spinner (`RiLoader4Line` with `animate-spin`) directly on the button provides clear context without shifting layout, keeping the user informed of the button's state.
