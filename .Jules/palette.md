## 2026-08-02 - Enhance Pagination Accessibility
**Learning:** Pagination components typically need aria-labels to provide meaningful context to screen-readers, especially on 'Next'/'Previous' and numerical page buttons, as well as an explicit semantic nav tag and aria-hidden on purely visual elements like ellipses.
**Action:** Use <nav aria-label='Pagination'> for the wrapper, specify aria-labels on numerical buttons and visual indicators on spacers.
