## 2024-05-18 - Replacing window.confirm with ConfirmDialog
**Learning:** Native `window.confirm` dialogs provide poor UX, look different on every browser, cannot be styled consistently with the design system, and create accessibility issues (like blocking the main thread and presenting focus challenges). The app already has a reusable `ConfirmDialog` component designed to replace it.
**Action:** Replace `window.confirm` calls with the design system's `ConfirmDialog` component across the application for consistent, accessible confirmations.
