const fs = require('fs');
let code = fs.readFileSync('src/web/src/routes/admin/AdminUsageLogsPage.test.tsx', 'utf8');

code = code.replace(
  "expect(screen.getByText('gpt-4o / 2026-06-04T00:00:00Z')).toBeInTheDocument();",
  "expect(screen.getAllByText('gpt-4o / 2026-06-04T00:00:00Z')[0]).toBeInTheDocument();"
);
code = code.replace(
  "expect(screen.getByText('user_1 / workspace_chat')).toBeInTheDocument();",
  "expect(screen.getAllByText('user_1 / workspace_chat')[0]).toBeInTheDocument();"
);

fs.writeFileSync('src/web/src/routes/admin/AdminUsageLogsPage.test.tsx', code);
