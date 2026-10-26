## 2024-10-26 - Tooltip Provider Wrapping
**Learning:** When adding tooltips to data tables with many rows (like AdminRoutesPage), wrapping the entire main container with `<TooltipProvider>` rather than wrapping individual components is more efficient and maintains cleaner component hierarchies.
**Action:** Default to wrapping the highest logical parent component (often the page root) with `<TooltipProvider>` when multiple tooltips are needed in lists or tables.
