## 2024-05-18 - Tooltips for icon-only action buttons in Datatable rows
**Learning:** Icon-only action buttons in datatable rows require tooltip feedback to be accessible and properly understood. In shadcn pattern for this repo, they must be wrapped with `<TooltipTrigger asChild>` inside `<Tooltip>` and `<TooltipProvider>` from `@/components/ui/tooltip`.
**Action:** Always wrap data-table action buttons in tooltips if they are represented solely by icons (e.g. `size="icon"` buttons).
