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

  describe('CRLF frame boundaries', () => {
    it('CRLF-delimited frames emit two ordered chunks instead of one merged frame', async () => {
      // Two complete CRLF-framed events in a single chunk.
      // Current parser uses indexOf('\n\n') which misses \r\n\r\n boundaries,
      // causing both frames to flush as one merged chunk.
      const fetchFn = vi.fn(async () =>
        new Response(
          streamFromChunks(['data: frame1\r\n\r\ndata: frame2\r\n\r\n']),
          { status: 200, headers: { 'Content-Type': 'text/event-stream' } }
        )
      );
      const chunks: string[] = [];

      await streamText(
        path,
        chunks.push.bind(chunks),
        streamMessageOperationContract,
        streamContract,
        fetchFn as unknown as typeof fetch,
        { body: '{}', method: 'POST' }
      );

      // Each CRLF-framed event must produce exactly one separate chunk.
      expect(chunks).toEqual(['frame1', 'frame2']);
    });

    it('CRLF boundary split across separate ReadableStream chunks is recognised after buffering', async () => {
      // Delimiter \r\n\r\n split so that \r\n arrives in chunk 1 and \r\n in chunk 2.
      // The incomplete suffix must remain buffered and the frame emitted only after
      // the second half of the delimiter arrives.
      const fetchFn = vi.fn(async () =>
        new Response(
          streamFromChunks(['data: frame1\r\n', '\r\ndata: frame2\r\n\r\n']),
          { status: 200, headers: { 'Content-Type': 'text/event-stream' } }
        )
      );
      const chunks: string[] = [];

      await streamText(
        path,
        chunks.push.bind(chunks),
        streamMessageOperationContract,
        streamContract,
        fetchFn as unknown as typeof fetch,
        { body: '{}', method: 'POST' }
      );

      expect(chunks).toEqual(['frame1', 'frame2']);
    });

    it('CRLF [DONE] terminates emission without consuming the preceding event as merged data', async () => {
      // When frames arrive in separate ReadableStream chunks the parser must
      // keep CRLF frames independent; [DONE] must not be merged with prior data.
      const fetchFn = vi.fn(async () =>
        new Response(
          streamFromChunks(['data: hello\r\n\r\n', 'data: [DONE]\r\n\r\n']),
          { status: 200, headers: { 'Content-Type': 'text/event-stream' } }
        )
      );
      const chunks: string[] = [];

      await streamText(
        path,
        chunks.push.bind(chunks),
        streamMessageOperationContract,
        streamContract,
        fetchFn as unknown as typeof fetch,
        { body: '{}', method: 'POST' }
      );

      // Only 'hello' must be emitted; [DONE] must not appear as a chunk value.
      expect(chunks).toEqual(['hello']);
    });

    it('CRLF event: error throws its data and calls onChunk zero times', async () => {
      // A preceding data frame followed by a CRLF error frame: both must be
      // parsed independently so only the error data reaches the thrown error.
      const fetchFn = vi.fn(async () =>
        new Response(
          streamFromChunks([
            'data: preamble\r\n\r\n',
            'event: error\r\ndata: quota preauthorization failed\r\n\r\n'
          ]),
          { status: 200, headers: { 'Content-Type': 'text/event-stream' } }
        )
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
      // The preamble data frame must have been emitted before the error frame.
      // After the error throws, onChunk must not have been called for the error itself.
      // Total onChunk calls = 1 (preamble only).
      expect(onChunk).toHaveBeenCalledTimes(1);
      expect(onChunk).toHaveBeenCalledWith('preamble');
    });

    it('LF-framed events continue to emit correctly (regression)', async () => {
      // Ensures that adding CRLF support does not break the established LF path.
      const fetchFn = vi.fn(async () =>
        new Response(
          streamFromChunks(['data: lf-frame1\n\n', 'data: lf-frame2\n\n']),
          { status: 200, headers: { 'Content-Type': 'text/event-stream' } }
        )
      );
      const chunks: string[] = [];

      await streamText(
        path,
        chunks.push.bind(chunks),
        streamMessageOperationContract,
        streamContract,
        fetchFn as unknown as typeof fetch,
        { body: '{}', method: 'POST' }
      );

      expect(chunks).toEqual(['lf-frame1', 'lf-frame2']);
    });
  });
});
