## 2025-01-20 - Adding Tooltips to Icon-Only Action Buttons

**Learning:** When dealing with multiple icon-only actions inside a data table column (e.g. `renderActions` in admin tables), wrapping each action button inside a `<Tooltip>` ensures keyboard users and people unfamiliar with the icons can easily understand the action. Icon-only buttons with just `aria-label` only solve for screen readers, not sighted users or those requiring explicit text indicators.
**Action:** Always wrap data-table action buttons (like Edit, Disable, Approve) inside a `<TooltipProvider>` and `<Tooltip>` when relying purely on icons like `RiPencilLine`.
