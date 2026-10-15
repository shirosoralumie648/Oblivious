## 2024-10-15 - Tooltips on Icon-Only Buttons
**Learning:** Found that while icon-only buttons in admin data tables have `aria-label`s for screen readers, they lack visual labels for sighted users who may not recognize the icons immediately. This is a common accessibility/usability gap where `aria-label` provides a11y but no visual cue.
**Action:** Always wrap icon-only buttons in `Tooltip` components from the design system (`@/components/ui/tooltip`) in addition to using `aria-label`. Ensure the button is wrapped in `TooltipTrigger asChild`.
