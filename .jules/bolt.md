## 2024-10-05 - Add React.memo to ConsoleOverviewCard
**Learning:** Component `ConsoleOverviewCard` receives simple props (`title`, `value`, `note`, `to`) and is rendered multiple times within a grid in `ConsoleHomePage`. Re-rendering `ConsoleHomePage` (e.g. when state changes) will unnecessarily re-render all instances of `ConsoleOverviewCard`.
**Action:** Use `React.memo()` for pure presentational components like cards or list items that frequently receive identical primitive props to prevent unnecessary DOM diffing and re-renders.
