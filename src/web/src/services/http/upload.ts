export async function uploadFile(
  path: string,
  file: File,
  fieldName = 'file',
  fetchFn: typeof fetch = fetch
): Promise<Response> {
  const formData = new FormData();
  formData.append(fieldName, file);

  return fetchFn(path, {
    method: 'POST',
    body: formData
  });
}
