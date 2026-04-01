export async function streamText(
  path: string,
  onChunk: (chunk: string) => void,
  fetchFn: typeof fetch = fetch
): Promise<void> {
  const response = await fetchFn(path);

  if (!response.ok || !response.body) {
    throw new Error('Unable to open stream');
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();

  while (true) {
    const { done, value } = await reader.read();

    if (done) {
      break;
    }

    onChunk(decoder.decode(value, { stream: true }));
  }
}
