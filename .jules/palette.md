## 2024-10-24 - Tooltips on icon-only buttons
**Learning:** Some icon-only buttons (like Edit/Disable User in AdminUsersPage) don't have tooltips which hurts accessibility for screen readers and usability for visually-abled users since icons can be ambiguous.
**Action:** Wrap icon-only buttons with the existing `Tooltip`, `TooltipTrigger`, and `TooltipContent` components from `@/components/ui/tooltip`. Make sure to import `TooltipProvider` and wrap the top level if not present, but AdminUsersPage doesn't have it, so we will wrap it at a higher level or around the page content.
