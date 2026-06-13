## 2024-06-13 - Focus management after actions

**Learning:** When clearing an input field via a button, the input field loses focus. Keyboard users must manually tab or click back into the input field.
**Action:** Use a `useRef` hook and call `focus()` to return focus to the input element programmatically after it's cleared.
