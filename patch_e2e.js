const fs = require('fs');
let code = fs.readFileSync('src/web/e2e/admin-api-tokens.spec.ts', 'utf8');

code = code.replace(
  `  page.once('dialog', async (dialog) => {
    expect(dialog.message()).toContain('Browser admin key');
    await dialog.accept();
  });
  await page.getByRole('button', { name: 'Revoke Browser admin key' }).click();`,
  `  await page.getByRole('button', { name: 'Revoke Browser admin key' }).click();
  await expect(page.getByText('Are you sure you want to revoke the API token "Browser admin key"? This action cannot be undone.')).toBeVisible();
  await page.getByRole('button', { name: 'Revoke Token' }).click();`
);

fs.writeFileSync('src/web/e2e/admin-api-tokens.spec.ts', code);
