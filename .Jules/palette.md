## 2024-05-24 - Accessible visual toggle buttons & mobile keyboards
**Learning:** For visual toggle buttons that act like a group but aren't native checkboxes or radio buttons (e.g. rating or price filters), `aria-pressed` must be applied to convey the active state to screen readers. For search inputs, using `type="search"` optimizes mobile keyboards by showing a "Search" action instead of "Return".
**Action:** Always verify active states of visual toggle filters map to `aria-pressed`, and ensure search fields have `type="search"` and an appropriate `aria-label`.
