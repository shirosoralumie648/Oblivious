const fs = require('fs');
let code = fs.readFileSync('src/web/src/routes/admin/AdminAPITokensPage.test.tsx', 'utf8');

code = code.replace(
  "const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);",
  ""
);

code = code.replace(
  "fireEvent.click(screen.getByRole('button', { name: 'Revoke Production key' }));",
  `fireEvent.click(screen.getByRole('button', { name: 'Revoke Production key' }));

    expect(screen.getByText('Are you sure you want to revoke the API token "Production key"? This action cannot be undone.')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Revoke Token' }));`
);

code = code.replace(
  "confirmSpy.mockRestore();",
  ""
);

fs.writeFileSync('src/web/src/routes/admin/AdminAPITokensPage.test.tsx', code);
