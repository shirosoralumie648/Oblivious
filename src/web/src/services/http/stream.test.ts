import { describe, expect, it, vi } from 'vitest';

import { streamText } from './stream';

function streamFromChunks(chunks: string[]) {
  const encoder = new TextEncoder();
  return new ReadableStream<Uint8Array>({
    start(controller) {
      for (const chunk of chunks) {
        controller.enqueue(encoder.encode(chunk));
      }
      controller.close();
    }
  });
}

describe('streamText', () => {
  it('posts request init and emits server-sent event data chunks', async () => {
    const fetchFn = vi.fn(async () =>
      new Response(streamFromChunks(['data: Hel', 'lo\n\n', 'data:  world\n\n', 'data: [DONE]\n\n']), {
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' }
      })
    );
    const chunks: string[] = [];

    await streamText('/api/v1/app/conversations/conversation_1/messages/stream', chunks.push.bind(chunks), fetchFn as unknown as typeof fetch, {
      body: JSON.stringify({ content: 'hello' }),
      headers: { 'Content-Type': 'application/json' },
      method: 'POST'
    });

    expect(fetchFn).toHaveBeenCalledWith('/api/v1/app/conversations/conversation_1/messages/stream', {
      body: JSON.stringify({ content: 'hello' }),
      headers: { 'Content-Type': 'application/json' },
      method: 'POST'
    });
    expect(chunks).toEqual(['Hello', ' world']);
  });

  it('throws when the stream cannot be opened', async () => {
    const fetchFn = vi.fn(async () => new Response('nope', { status: 500 }));

    await expect(streamText('/stream', () => undefined, fetchFn as unknown as typeof fetch)).rejects.toThrow('Unable to open stream');
  });

  it('throws server-sent event errors instead of emitting them as chunks', async () => {
    const fetchFn = vi.fn(async () =>
      new Response(streamFromChunks(['event: error\n', 'data: quota preauthorization failed\n\n']), {
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' }
      })
    );
    const onChunk = vi.fn();

    await expect(streamText('/stream', onChunk, fetchFn as unknown as typeof fetch)).rejects.toThrow(
      'quota preauthorization failed'
    );
    expect(onChunk).not.toHaveBeenCalled();
  });
});
