
## 2024-05-24 - Memoize component calculations depending on large dataset
**Learning:** Component calculations that map/filter over arrays should be memoized, especially if they are re-evaluated frequently (e.g., selection toggles in tables). Without `useMemo`, mapping over `data` to calculate row IDs on every render (such as on checkbox tick) hurts performance when the data set is large.
**Action:** Use `useMemo` for derived dataset arrays, and add `data` to its dependency array, so we don't recalculate unless the data actually changes.
