## 2024-05-15 - Improve Pagination Accessibility
**Learning:** Pure semantic accessibility enhancements (changing div to nav, adding aria-labels) can be fully implemented without modifying existing css classes or breaking component structures, ensuring complete visual regression safety. The `Pagination` component now natively announces page boundaries correctly.
**Action:** Always prefer native semantic tags (like `nav` for pagination groups) and utilize `aria-hidden="true"` for non-interactive visual spacers like ellipses to immediately reduce screen reader noise.
