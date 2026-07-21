import { describe, expect, it, vi } from 'vitest';

import {
  exportConversationMarkdownOperationContract,
  uploadKnowledgeDocumentOperationContract
} from '@/generated/operation-contracts.generated';

import { formDataRequestEncoder, rawResponseDecoder } from './client';
import { uploadFile } from './upload';

const uploadContract = {
  operation: uploadKnowledgeDocumentOperationContract,
  requestEncoder: formDataRequestEncoder(uploadKnowledgeDocumentOperationContract),
  responseDecoder: rawResponseDecoder(uploadKnowledgeDocumentOperationContract, 200)
};

describe('uploadFile', () => {
  const path = '/api/v1/app/knowledge-bases/kb_1/documents/upload';

  it('uploads FormData with exact operation metadata and no manual multipart boundary', async () => {
    const response = new Response(JSON.stringify({ ok: true, data: { id: 'doc_1' }, error: null }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' }
    });
    const fetchFn = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => response);
    const file = new File(['# Notes'], 'notes.md', { type: 'text/markdown' });

    await expect(
      uploadFile(
        path,
        file,
        uploadKnowledgeDocumentOperationContract,
        uploadContract,
        'document',
        fetchFn as unknown as typeof fetch
      )
    ).resolves.toBe(response);

    const [, init] = fetchFn.mock.calls[0];
    expect(init).toMatchObject({ method: 'POST', headers: { Accept: 'application/json' } });
    expect(init?.headers).not.toHaveProperty('Content-Type');
    expect(init?.body).toBeInstanceOf(FormData);
    expect((init?.body as FormData).get('document')).toBe(file);
  });

  it('rejects wrong operation metadata before creating a network effect', async () => {
    const fetchFn = vi.fn();
    const file = new File(['# Notes'], 'notes.md', { type: 'text/markdown' });

    await expect(
      uploadFile(
        path,
        file,
        exportConversationMarkdownOperationContract,
        uploadContract,
        'file',
        fetchFn as unknown as typeof fetch
      )
    ).rejects.toThrow('does not reference the caller-supplied operation metadata');
    expect(fetchFn).not.toHaveBeenCalled();
  });
});
