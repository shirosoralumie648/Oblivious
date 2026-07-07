Fixed flaky test in console-overview.spec.ts where a locator was incorrectly resolving to multiple elements because it didn't use `exact: true`.
Fixed flaky React Flow test in WorkflowsPage.reactflow.test.tsx by wrapping `expect(canvas.querySelector('[data-id="manual-start"]')).toBeInTheDocument()` in a `waitFor` call. React Flow nodes take a small amount of time to render on the canvas.
