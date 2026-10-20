## 2024-10-20 - Add tooltips to icon-only action buttons
**Learning:** Administrative tables with icon-only action buttons lack discoverability for sighted users who may not recognize the icons, even if ARIA labels are present. Adding standard tooltips significantly improves UX by surfacing the action intent on hover/focus.
**Action:** When implementing icon-only buttons in DataTables, always wrap them in the standard `TooltipProvider` -> `Tooltip` -> `TooltipTrigger` -> `TooltipContent` pattern to ensure intent is clear to all users.
