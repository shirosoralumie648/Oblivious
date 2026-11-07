## 2025-01-20 - Tooltips on DataTable Action Buttons
**Learning:** Icon-only action buttons inside DataTables are difficult for sighted users to decipher without hovering text, even with `aria-label`. The `TooltipProvider` must be hoisted to the page root, wrapping the `DataTable`, to avoid unnecessary repeated context providers per row.
**Action:** Always wrap the main page element with a single `TooltipProvider` when a data table uses row-level tooltips.
