## 2024-06-15 - Contextual ARIA labels for repetitive actions in lists
**Learning:** Adding specific context (like the item name) to repetitive buttons (e.g. "Connect", "Disconnect", "Diagnose") in a grid or list is crucial. Screen reader users navigating by interactive elements might otherwise encounter indistinguishable rows of identical labels.
**Action:** When creating repetitive list items with action buttons, always ensure each button's `aria-label` incorporates the name or unique identifier of the corresponding item.
