## 2024-10-23 - Add Tooltips to AdminRoutesPage Icon Buttons
**Learning:** Icon-only buttons for actions like Edit/Delete/Remove Target lack contextual text, which is an accessibility issue.
**Action:** Always wrap icon-only action buttons in the existing `Tooltip` system (`TooltipProvider`, `Tooltip`, `TooltipTrigger`, `TooltipContent`) to ensure functionality is accessible and discoverable.
