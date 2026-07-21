import { describe, expect, it, vi } from 'vitest';

import {
  exportConversationMarkdownOperationContract,
  streamMessageOperationContract
} from '@/generated/operation-contracts.generated';

import { jsonRequestEncoder, rawResponseDecoder } from './client';
import { streamText } from './stream';

const streamContract = {
  operation: streamMessageOperationContract,
  requestEncoder: jsonRequestEncoder(streamMessageOperationContract),
  responseDecoder: rawResponseDecoder(streamMessageOperationContract, 200)
};

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
  const path = '/api/v1/app/conversations/conversation_1/messages/stream';

  it('posts exact operation metadata and emits server-sent event data chunks', async () => {
    const fetchFn = vi.fn(async () =>
      new Response(streamFromChunks(['data: Hel', 'lo\n\n', 'data:  world\n\n', 'data: [DONE]\n\n']), {
        status: 200,
        headers: { 'Content-Type': 'text/event-stream; charset=utf-8' }
      })
    );
    const chunks: string[] = [];

    await streamText(
      path,
      chunks.push.bind(chunks),
      streamMessageOperationContract,
      streamContract,
      fetchFn as unknown as typeof fetch,
      {
        body: JSON.stringify({ content: 'hello' }),
        headers: { 'Content-Type': 'application/json' },
        method: 'POST'
      }
    );

    expect(fetchFn).toHaveBeenCalledWith(path, {
      body: JSON.stringify({ content: 'hello' }),
      headers: { Accept: 'text/event-stream', 'Content-Type': 'application/json' },
      method: 'POST'
    });
    expect(chunks).toEqual(['Hello', ' world']);
  });

  it('rejects wrong operation metadata before opening the transport', async () => {
    const fetchFn = vi.fn();

    await expect(
      streamText(
        path,
        () => undefined,
        exportConversationMarkdownOperationContract,
        streamContract,
        fetchFn as unknown as typeof fetch,
        { body: '{}', method: 'POST' }
      )
    ).rejects.toThrow('does not reference the caller-supplied operation metadata');
    expect(fetchFn).not.toHaveBeenCalled();
  });

  it('throws when the validated stream has no body', async () => {
    const fetchFn = vi.fn(async () =>
      new Response(null, { status: 200, headers: { 'Content-Type': 'text/event-stream' } })
    );

    await expect(
      streamText(
        path,
        () => undefined,
        streamMessageOperationContract,
        streamContract,
        fetchFn as unknown as typeof fetch,
        { body: '{}', method: 'POST' }
      )
    ).rejects.toThrow('Unable to open stream');
  });

  it('throws server-sent event errors instead of emitting them as chunks', async () => {
    const fetchFn = vi.fn(async () =>
      new Response(streamFromChunks(['event: error\n', 'data: quota preauthorization failed\n\n']), {
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' }
      })
    );
    const onChunk = vi.fn();

    await expect(
      streamText(
        path,
        onChunk,
        streamMessageOperationContract,
        streamContract,
        fetchFn as unknown as typeof fetch,
        { body: '{}', method: 'POST' }
      )
    ).rejects.toThrow('quota preauthorization failed');
    expect(onChunk).not.toHaveBeenCalled();
  });
});
