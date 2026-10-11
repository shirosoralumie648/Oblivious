## 2024-03-20 - Add tooltips to icon-only data table buttons
**Learning:** Wrapping icon-only actions inside `DataTable` rows with `TooltipProvider` -> `Tooltip` -> `TooltipTrigger` -> `TooltipContent` provides essential visual context for sighted users who don't read the `aria-label`, improving overall discoverability of actions.
**Action:** Whenever adding generic icon buttons (edit, delete, disable) in data tables, immediately wrap them in tooltips instead of relying solely on `aria-label`.
