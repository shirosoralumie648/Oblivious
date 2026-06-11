## 2024-05-24 - Async Button Loading States
**Learning:** Adding visual loading spinners to async buttons in data-heavy panels like `McpServersPanel` significantly improves perceived responsiveness and user confidence. Disabling the button alone without a spinner leaves the user wondering if their click registered.
**Action:** When adding or modifying buttons that trigger async actions, use the existing pattern of conditionally rendering an `<RiLoader4Line className="animate-spin" />` icon in place of the standard icon while the action is in progress.
