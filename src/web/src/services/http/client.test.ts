import { describe, expect, it, vi } from 'vitest';

import {
  deleteKnowledgeBaseOperationContract,
  exportConversationMarkdownOperationContract,
  updateConversationConfigOperationContract
} from '@/generated/operation-contracts.generated';

import {
  createHttpClient,
  jsonEnvelopeDecoder,
  jsonRequestEncoder,
  noneResponseDecoder,
  noneRequestEncoder,
  rawResponseDecoder,
  textResponseDecoder
} from './client';

describe('http client', () => {
  it('unwraps successful envelope payloads with JSON media validation', async () => {
    const fetchFn = vi.fn(async () =>
      new Response(JSON.stringify({ ok: true, data: { requests: 3 }, error: null }), {
        status: 200,
        headers: { 'Content-Type': 'application/json; charset=utf-8' }
      })
    );

    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });
    const result = await client.get<{ requests: number }>('/api/v1/console/usage');

    expect(result).toEqual({ requests: 3 });
    expect(fetchFn).toHaveBeenCalledWith('/api/v1/console/usage', {
      body: undefined,
      method: 'GET',
      headers: { Accept: 'application/json' }
    });
  });

  it('selects JSON request and response codecs from exact operation metadata', async () => {
    const fetchFn = vi.fn(async () =>
      new Response(JSON.stringify({ ok: true, data: { mode: 'solo' }, error: null }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' }
      })
    );
    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });

    await expect(
      client.put(
        '/api/v1/app/conversations/conversation_1/config',
        { mode: 'solo' },
        undefined,
        {
          operation: updateConversationConfigOperationContract,
          requestEncoder: jsonRequestEncoder(updateConversationConfigOperationContract),
          responseDecoder: jsonEnvelopeDecoder(updateConversationConfigOperationContract, 200)
        }
      )
    ).resolves.toEqual({ mode: 'solo' });

    expect(fetchFn).toHaveBeenCalledWith('/api/v1/app/conversations/conversation_1/config', {
      body: JSON.stringify({ mode: 'solo' }),
      method: 'PUT',
      headers: { Accept: 'application/json', 'Content-Type': 'application/json' }
    });
  });

  it('decodes text media with charset without touching JSON', async () => {
    const response = new Response('# Launch Review\n', {
      status: 200,
      headers: { 'Content-Type': 'Text/Markdown; Charset=utf-8' }
    });
    const jsonSpy = vi.spyOn(response, 'json');
    const fetchFn = vi.fn(async () => response);
    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });

    await expect(
      client.get('/api/v1/app/conversations/conversation_1/export.md', undefined, {
        operation: exportConversationMarkdownOperationContract,
        requestEncoder: noneRequestEncoder(exportConversationMarkdownOperationContract),
        responseDecoder: textResponseDecoder(exportConversationMarkdownOperationContract, 200)
      })
    ).resolves.toBe('# Launch Review\n');

    expect(jsonSpy).not.toHaveBeenCalled();
    expect(fetchFn).toHaveBeenCalledWith('/api/v1/app/conversations/conversation_1/export.md', {
      body: undefined,
      method: 'GET',
      headers: { Accept: 'text/markdown' }
    });
  });

  it('rejects contradictory Accept and wrong success media before decoding', async () => {
    const fetchFn = vi.fn(async () =>
      new Response('# Launch Review\n', { status: 200, headers: { 'Content-Type': 'application/json' } })
    );
    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });
    const contract = {
      operation: exportConversationMarkdownOperationContract,
      requestEncoder: noneRequestEncoder(exportConversationMarkdownOperationContract),
      responseDecoder: textResponseDecoder(exportConversationMarkdownOperationContract, 200)
    };

    await expect(
      client.get('/api/v1/app/conversations/conversation_1/export.md', { headers: { Accept: 'application/json' } }, contract)
    ).rejects.toThrow('contradicts declared response media type');
    expect(fetchFn).not.toHaveBeenCalled();

    await expect(client.get('/api/v1/app/conversations/conversation_1/export.md', undefined, contract)).rejects.toThrow(
      'Response media type does not match'
    );
  });

  it('rejects mismatched schema identity and operation status before transport', async () => {
    const fetchFn = vi.fn();
    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });
    const decoder = textResponseDecoder(exportConversationMarkdownOperationContract, 200);

    await expect(
      client.get('/api/v1/app/conversations/conversation_1/export.md', undefined, {
        operation: exportConversationMarkdownOperationContract,
        requestEncoder: {
          ...noneRequestEncoder(exportConversationMarkdownOperationContract),
          schemaIdentity: { ...exportConversationMarkdownOperationContract.request.schemaIdentity }
        },
        responseDecoder: decoder
      })
    ).rejects.toThrow('Request encoder identity does not match');

    await expect(
      client.get('/api/v1/app/conversations/conversation_1/export.md', undefined, {
        operation: exportConversationMarkdownOperationContract,
        requestEncoder: noneRequestEncoder(exportConversationMarkdownOperationContract),
        responseDecoder: { ...decoder, status: 201 }
      })
    ).rejects.toThrow('does not declare one success response for status 201');
    expect(fetchFn).not.toHaveBeenCalled();
  });

  it('uses none semantics for legacy 204 responses without decoding a body', async () => {
    const response = new Response(null, { status: 204 });
    const jsonSpy = vi.spyOn(response, 'json');
    const textSpy = vi.spyOn(response, 'text');
    const client = createHttpClient({ fetchFn: vi.fn(async () => response) as unknown as typeof fetch });

    await expect(client.delete<void>('/api/v1/app/knowledge-bases/kb_1')).resolves.toBeUndefined();
    expect(jsonSpy).not.toHaveBeenCalled();
    expect(textSpy).not.toHaveBeenCalled();
  });

  it('uses an explicit none decoder for declared 204 responses without an Accept header', async () => {
    const response = new Response(null, { status: 204 });
    const jsonSpy = vi.spyOn(response, 'json');
    const textSpy = vi.spyOn(response, 'text');
    const fetchFn = vi.fn(async () => response);
    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });

    await expect(
      client.delete('/api/v1/app/knowledge-bases/kb_1', undefined, {
        operation: deleteKnowledgeBaseOperationContract,
        requestEncoder: noneRequestEncoder(deleteKnowledgeBaseOperationContract),
        responseDecoder: noneResponseDecoder(deleteKnowledgeBaseOperationContract, 204)
      })
    ).resolves.toBeUndefined();

    expect(jsonSpy).not.toHaveBeenCalled();
    expect(textSpy).not.toHaveBeenCalled();
    expect(fetchFn).toHaveBeenCalledWith('/api/v1/app/knowledge-bases/kb_1', {
      body: undefined,
      method: 'DELETE',
      headers: {}
    });
  });

  it('returns the original Response through a declared raw decoder', async () => {
    const response = new Response('# Launch Review\n', {
      status: 200,
      headers: { 'Content-Type': 'text/markdown' }
    });
    const client = createHttpClient({ fetchFn: vi.fn(async () => response) as unknown as typeof fetch });

    await expect(
      client.get<Response>('/api/v1/app/conversations/conversation_1/export.md', undefined, {
        operation: exportConversationMarkdownOperationContract,
        requestEncoder: noneRequestEncoder(exportConversationMarkdownOperationContract),
        responseDecoder: rawResponseDecoder(exportConversationMarkdownOperationContract, 200)
      })
    ).resolves.toBe(response);
    expect(response.bodyUsed).toBe(false);
  });

  it('rejects missing JSON media and malformed decoders before returning data', async () => {
    const missingMediaClient = createHttpClient({
      fetchFn: vi.fn(async () => new Response(JSON.stringify({ ok: true, data: {}, error: null }), { status: 200 })) as unknown as typeof fetch
    });
    await expect(missingMediaClient.get('/api/v1/console/usage')).rejects.toThrow(
      'missing a JSON-compatible Content-Type'
    );

    const fetchFn = vi.fn();
    const malformedClient = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });
    const decoder = textResponseDecoder(exportConversationMarkdownOperationContract, 200);
    await expect(
      malformedClient.get('/api/v1/app/conversations/conversation_1/export.md', undefined, {
        operation: exportConversationMarkdownOperationContract,
        requestEncoder: noneRequestEncoder(exportConversationMarkdownOperationContract),
        responseDecoder: { ...decoder, decode: null } as unknown as typeof decoder
      })
    ).rejects.toThrow('Response decoder identity does not match');
    expect(fetchFn).not.toHaveBeenCalled();
  });

  it('preserves structured JSON errors and redacts non-JSON bodies', async () => {
    const structured = new Response(
      JSON.stringify({
        ok: false,
        data: { decision: 'rejected' },
        error: { code: 'automated_review_rejected', message: 'Automated review rejected publication.' }
      }),
      { status: 400, statusText: 'Bad Request', headers: { 'Content-Type': 'application/problem+json' } }
    );
    const raw = new Response('provider-secret-body', { status: 502, statusText: 'Bad Gateway' });
    const fetchFn = vi.fn().mockResolvedValueOnce(structured).mockResolvedValueOnce(raw);
    const client = createHttpClient({ fetchFn: fetchFn as unknown as typeof fetch });

    await expect(client.post('/api/v1/marketplace/agents', { name: 'Unsafe Agent' })).rejects.toMatchObject({
      status: 400,
      code: 'automated_review_rejected',
      message: 'Automated review rejected publication.',
      data: { decision: 'rejected' }
    });
    await expect(client.get('/api/v1/console/usage')).rejects.toEqual(
      expect.objectContaining({ status: 502, message: 'Bad Gateway' })
    );
    expect(String(await raw.clone().text())).toContain('provider-secret-body');
  });

  it('sends multipart request bodies without a JSON content type', async () => {
    const fetchFn = vi.fn(async () =>
      new Response(JSON.stringify({ ok: true, data: { uploaded: true }, error: null }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' }
      })
    );
    const client = createHttpClient({ baseUrl: 'https://api.example.test', fetchFn: fetchFn as unknown as typeof fetch });
    const formData = new FormData();
    formData.append('file', new File(['body'], 'notes.md', { type: 'text/markdown' }));

    const result = await client.request<{ uploaded: boolean }>('/api/v1/app/knowledge-bases/kb_1/documents/upload', {
      body: formData,
      method: 'POST'
    });

    expect(result).toEqual({ uploaded: true });
    expect(fetchFn).toHaveBeenCalledWith('https://api.example.test/api/v1/app/knowledge-bases/kb_1/documents/upload', {
      body: formData,
      method: 'POST',
      headers: { Accept: 'application/json' }
    });
  });
});
