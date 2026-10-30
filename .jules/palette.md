## 2026-10-30 - Wrap TooltipProvider at the highest logical parent component
**Learning:** For data tables or lists with multiple tooltips, wrap the highest logical parent component (e.g., the page root) with a single `<TooltipProvider>` rather than wrapping individual components.
**Action:** Use a single `<TooltipProvider>` at the page root level instead of wrapping each `<Tooltip>` individually.
