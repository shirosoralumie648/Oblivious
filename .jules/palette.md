## 2024-05-24 - Wrapped Data Table Action Buttons with Tooltips
**Learning:** Discovered that icon-only action buttons in data tables lack accessible tooltips, but the design system pattern requires wrapping the highest logical parent component (e.g., page root) with `<TooltipProvider>` rather than individual components to support multiple tooltips efficiently.
**Action:** When adding tooltips to data tables or lists, always wrap the page root with `<TooltipProvider>` and use `<TooltipTrigger asChild>` with `<TooltipContent>` for individual buttons.
