export async function streamText(
  path: string,
  onChunk: (chunk: string) => void,
  fetchFn: typeof fetch = fetch,
  init?: RequestInit
): Promise<void> {
  const response = await fetchFn(path, init);

  if (!response.ok || !response.body) {
    throw new Error('Unable to open stream');
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();

    if (done) {
      break;
    }

    buffer += decoder.decode(value, { stream: true });
    buffer = emitBufferedChunks(buffer, onChunk);
  }

  buffer += decoder.decode();
  emitBufferedChunks(buffer, onChunk, true);
}

function emitBufferedChunks(buffer: string, onChunk: (chunk: string) => void, flush = false) {
  let remaining = buffer;

  while (true) {
    const boundary = remaining.indexOf('\n\n');
    if (boundary === -1) {
      break;
    }
    const frame = remaining.slice(0, boundary);
    remaining = remaining.slice(boundary + 2);
    emitFrame(frame, onChunk);
  }

  if (flush && remaining.trim() !== '') {
    emitFrame(remaining, onChunk);
    return '';
  }

  return remaining;
}

function emitFrame(frame: string, onChunk: (chunk: string) => void) {
  const lines = frame.split(/\r?\n/);
  const eventType = lines
    .find((line) => line.startsWith('event:'))
    ?.slice(6)
    .trim();
  const dataLines = lines
    .filter((line) => line.startsWith('data:'))
    .map((line) => {
      const value = line.slice(5);
      return value.startsWith(' ') ? value.slice(1) : value;
    });

  if (dataLines.length === 0) {
    if (frame.trim() !== '') {
      onChunk(frame);
    }
    return;
  }

  const data = dataLines.join('\n');
  if (data === '[DONE]') {
    return;
  }
  const normalizedData = data.replace(/\\n/g, '\n');
  if (eventType === 'error') {
    throw new Error(normalizedData || 'Stream returned an error');
  }
  onChunk(normalizedData);
}
