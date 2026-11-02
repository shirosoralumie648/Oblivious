
## 2026-11-02 - Data Table Action Tooltips
**Learning:** Tooltips on icon-only buttons in data tables require wrapping the root page component in `<TooltipProvider>` to avoid unnecessary repeated context providers per row, then wrapping the individual buttons in `<Tooltip>` and `<TooltipTrigger>`.
**Action:** Always wrap data tables that contain multiple tooltips with a single high-level `TooltipProvider` instead of wrapping each element.
