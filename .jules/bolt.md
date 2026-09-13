## 2024-05-24 - [Memoize Selectable Rows in Shared DataTable]
**Learning:** Calculating state properties (like mapping rows to extract IDs) in a highly re-used UI component when its associated feature (like row selection) is globally disabled for that instance adds an unnecessary performance tax that scales with the table size across the entire application.
**Action:** Use `useMemo` combined with a feature toggle check (`selectable ? data.map(...) : []`) to completely bypass expensive array transformations when the feature isn't active for the current component instance.
