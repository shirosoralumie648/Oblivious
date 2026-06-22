## 2024-06-22 - Missing Disabled States During Async Actions on Admin Action Buttons
**Learning:** Icon buttons with loading spinners in `AdminChannelsPage.tsx` were previously missing the `disabled` attribute, meaning users could accidentally trigger multiple API calls.
**Action:** Always ensure `disabled` is tied to the exact same loading state condition as the visual spinner to prevent double-submissions.
