import { describe, expect, it, vi } from 'vitest';

import type { HttpClient } from '../../services/http/client';
import { createKnowledgeApi } from './api';

function createClient(overrides: Partial<HttpClient> = {}) {
  const client: HttpClient = {
    delete: vi.fn(),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    request: vi.fn(),
    ...overrides
  };
  return client;
}

describe('createKnowledgeApi', () => {
  it('lists and runs retrieval test cases for a knowledge base', async () => {
    const testCases = [
      {
        expectedChunkId: 'kdc_7',
        expectedChunkIndex: 4,
        expectedDocumentId: 'doc_1',
        id: 'krtc_1',
        knowledgeBaseId: 'kb_9',
        query: 'deployment rollback'
      }
    ];
    const report = {
      failed: 0,
      knowledgeBaseId: 'kb_9',
      passed: 1,
      ranAt: '2026-06-05T12:00:00Z',
      results: [
        {
          expectedResult: {
            chunkId: 'kdc_7',
            chunkIndex: 4,
            documentId: 'doc_1',
            documentTitle: 'Overview',
            retrievalMethod: 'hybrid',
            similarity: 0.87,
            snippet: 'Deployment rollback plans belong in the release runbook.',
            source: {
              chunkId: 'kdc_7',
              chunkIndex: 4,
              documentId: 'doc_1',
              documentTitle: 'Overview'
            }
          },
          passed: true,
          query: 'deployment rollback',
          rank: 1,
          testCaseId: 'krtc_1'
        }
      ],
      total: 1
    };
    const get = vi.fn().mockResolvedValue(testCases);
    const post = vi.fn().mockResolvedValue(report);
    const api = createKnowledgeApi(createClient({ get, post }));

    await expect(api.listRetrievalTestCases('kb_9')).resolves.toEqual(testCases);
    await expect(api.runRetrievalTestCases('kb_9', { limit: 3, mode: 'hybrid' })).resolves.toEqual(report);

    expect(get).toHaveBeenCalledWith('/api/v1/app/knowledge-bases/kb_9/retrieval-test-cases');
    expect(post).toHaveBeenCalledWith('/api/v1/app/knowledge-bases/kb_9/retrieval-test-cases/run', {
      limit: 3,
      mode: 'hybrid'
    });
  });

  it('creates retrieval test cases from scored retrieval results', async () => {
    const created = {
      expectedChunkId: 'kdc_7',
      expectedChunkIndex: 4,
      expectedDocumentId: 'doc_1',
      id: 'krtc_1',
      knowledgeBaseId: 'kb_9',
      query: 'deployment rollback'
    };
    const post = vi.fn().mockResolvedValue(created);
    const api = createKnowledgeApi(createClient({ post }));
    const payload = {
      expectedResult: {
        chunkId: 'kdc_7',
        chunkIndex: 4,
        documentId: 'doc_1',
        documentTitle: 'Overview',
        retrievalMethod: 'hybrid',
        similarity: 0.87,
        snippet: 'Deployment rollback plans belong in the release runbook.',
        source: {
          chunkId: 'kdc_7',
          chunkIndex: 4,
          documentId: 'doc_1',
          documentTitle: 'Overview'
        }
      },
      query: 'deployment rollback'
    };

    await expect(api.createRetrievalTestCase('kb_9', payload)).resolves.toEqual(created);

    expect(post).toHaveBeenCalledWith('/api/v1/app/knowledge-bases/kb_9/retrieval-test-cases', payload);
  });

  it('updates a knowledge document chunk with edited content', async () => {
    const updatedChunk = {
      charCount: 28,
      chunkId: 'kdc_7',
      chunkIndex: 2,
      content: 'Edited architecture chunk.',
      documentVersion: 'v3',
      estimatedTokenCount: 7,
      metadata: {
        documentVersion: 'v3'
      }
    };
    const put = vi.fn().mockResolvedValue(updatedChunk);
    const api = createKnowledgeApi(createClient({ put }));

    await expect(api.updateKnowledgeDocumentChunk('kb_9', 'doc_1', 'kdc_7', { content: 'Edited architecture chunk.' })).resolves.toEqual(
      updatedChunk
    );

    expect(put).toHaveBeenCalledWith('/api/v1/app/knowledge-bases/kb_9/documents/doc_1/chunks/kdc_7', {
      content: 'Edited architecture chunk.'
    });
  });

  it('splits and merges knowledge document chunks', async () => {
    const chunks = [
      {
        charCount: 12,
        chunkId: 'kdc_left',
        chunkIndex: 0,
        content: 'Left chunk.',
        documentVersion: 'v3',
        estimatedTokenCount: 3,
        metadata: {
          documentVersion: 'v3'
        }
      }
    ];
    const post = vi.fn().mockResolvedValue(chunks);
    const api = createKnowledgeApi(createClient({ post }));

    await expect(api.splitKnowledgeDocumentChunk('kb_9', 'doc_1', 'kdc_7', { splitAt: 18 })).resolves.toEqual(chunks);
    await expect(api.mergeKnowledgeDocumentChunks('kb_9', 'doc_1', 'kdc_7', { direction: 'next' })).resolves.toEqual(chunks);

    expect(post).toHaveBeenNthCalledWith(1, '/api/v1/app/knowledge-bases/kb_9/documents/doc_1/chunks/kdc_7/split', {
      splitAt: 18
    });
    expect(post).toHaveBeenNthCalledWith(2, '/api/v1/app/knowledge-bases/kb_9/documents/doc_1/chunks/kdc_7/merge', {
      direction: 'next'
    });
  });

  it('uploads knowledge documents through multipart form data', async () => {
    const uploadedDocument = {
      content: 'Uploaded content',
      documentVersion: 'v4',
      id: 'doc_upload',
      title: 'Uploaded Runbook',
      updateStrategy: 'versioned' as const,
      updatedAt: '2026-04-03T12:45:00Z'
    };
    const request = vi.fn().mockResolvedValue(uploadedDocument);
    const api = createKnowledgeApi(createClient({ request }));
    const file = new File(['uploaded body'], 'runbook.md', { type: 'text/markdown' });

    await expect(
      api.uploadKnowledgeDocument('kb_9', {
        documentVersion: 'v4',
        file,
        pageNumber: 12,
        sourceUrl: ' https://docs.example/runbook.md ',
        title: 'Uploaded Runbook',
        updateStrategy: 'versioned'
      })
    ).resolves.toEqual(uploadedDocument);

    expect(request).toHaveBeenCalledWith('/api/v1/app/knowledge-bases/kb_9/documents/upload', {
      body: expect.any(FormData),
      method: 'POST'
    });
    const formData = request.mock.calls[0][1].body as FormData;
    expect(formData.get('file')).toBe(file);
    expect(formData.get('title')).toBe('Uploaded Runbook');
    expect(formData.get('documentVersion')).toBe('v4');
    expect(formData.get('pageNumber')).toBe('12');
    expect(formData.get('sourceUrl')).toBe('https://docs.example/runbook.md');
    expect(formData.get('updateStrategy')).toBe('versioned');
  });
});
