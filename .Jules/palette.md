## 2024-11-20 - Replacing window.confirm with ConfirmDialog
**Learning:** Using native window.confirm provides a poor, blocking UX and fails accessibility/design system standards. In React, replacing it with a custom UI Dialog component improves a11y and consistency, but breaks tests that rely on handling native dialog events.
**Action:** Use ConfirmDialog for destructive confirmations and update Playwright E2E tests to interact with the DOM rather than listening for native dialog events.
