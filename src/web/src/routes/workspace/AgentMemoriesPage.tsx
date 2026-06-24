import { useMemo, useState } from 'react';

import { createAgentMemoriesApi, type AgentMemory } from '../../features/agents/memoriesApi';
import { createHttpClient } from '../../services/http/client';

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim() !== '') {
    return error.message;
  }
  if (typeof error === 'string' && error.trim() !== '') {
    return error;
  }
  return fallback;
}

function memoryTypeLabel(type: string) {
  switch (type) {
    case 'short_term':
      return 'Short term';
    case 'user_managed':
      return 'User managed';
    case 'long_term':
      return 'Long term';
    default:
      return type || 'Memory';
  }
}

function formatMetadata(metadata: Record<string, unknown>) {
  return Object.entries(metadata)
    .map(([key, value]) => `${key}=${typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' ? String(value) : JSON.stringify(value)}`)
    .join(', ');
}

function normalizeImportance(importance: number | undefined) {
  if (typeof importance === 'number' && Number.isFinite(importance) && importance >= 1 && importance <= 5) {
    return Math.trunc(importance);
  }
  return 3;
}

function memoryExportPayload(memories: AgentMemory[]) {
  return memories.map((memory) => ({
    agentId: memory.agentId,
    content: memory.content,
    importance: normalizeImportance(memory.importance),
    metadata: memory.metadata,
    type: memory.type || 'user_managed'
  }));
}

function importableMemoryPayload(value: unknown) {
  if (typeof value !== 'object' || value === null || !('content' in value) || typeof value.content !== 'string') {
    return null;
  }
  const contentValue = value.content;
  const source = value as Partial<AgentMemory>;
  const content = contentValue.trim();
  if (content === '') {
    return null;
  }
  const metadata =
    source.metadata && typeof source.metadata === 'object' && !Array.isArray(source.metadata)
      ? (source.metadata as Record<string, unknown>)
      : { importedBy: 'workspace_import' };

  return {
    agentId: typeof source.agentId === 'string' && source.agentId.trim() !== '' ? source.agentId.trim() : undefined,
    content,
    importance: normalizeImportance(source.importance),
    metadata,
    type: source.type || 'user_managed'
  };
}

function readFileAsText(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.addEventListener('error', () => reject(reader.error ?? new Error('Unable to read import file.')));
    reader.addEventListener('load', () => resolve(typeof reader.result === 'string' ? reader.result : ''));
    reader.readAsText(file);
  });
}

function ImportanceBadge({ importance }: { importance: number | undefined }) {
  const value = normalizeImportance(importance);
  return (
    <span aria-label={`Importance ${value} of 5`} className="inline-flex items-center gap-1">
      <span aria-hidden="true">{'★'.repeat(value)}{'☆'.repeat(5 - value)}</span>
    </span>
  );
}

export function AgentMemoriesPage() {
  const memoriesApi = useMemo(() => createAgentMemoriesApi(createHttpClient()), []);
  const [agentId, setAgentId] = useState('');
  const [content, setContent] = useState('');
  const [createImportance, setCreateImportance] = useState('3');
  const [editingContent, setEditingContent] = useState('');
  const [editingImportance, setEditingImportance] = useState('3');
  const [editingMemoryId, setEditingMemoryId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isCreating, setIsCreating] = useState(false);
  const [savingMemoryId, setSavingMemoryId] = useState<string | null>(null);
  const [deletingMemoryId, setDeletingMemoryId] = useState<string | null>(null);
  const [exportUrl, setExportUrl] = useState('');
  const [isExporting, setIsExporting] = useState(false);
  const [isImporting, setIsImporting] = useState(false);
  const [isSearching, setIsSearching] = useState(false);
  const [memories, setMemories] = useState<AgentMemory[]>([]);
  const [memoryType, setMemoryType] = useState('');
  const [query, setQuery] = useState('');
  const [resultLimit, setResultLimit] = useState('10');
  const [total, setTotal] = useState(0);

  const createMemory = async () => {
    const trimmedContent = content.trim();
    if (trimmedContent === '') {
      return;
    }

    setIsCreating(true);
    setError(null);

    try {
      const parsedImportance = Number.parseInt(createImportance, 10);
      const created = await memoriesApi.createMemory({
        agentId: agentId.trim() || undefined,
        content: trimmedContent,
        importance: Number.isFinite(parsedImportance) ? parsedImportance : 3,
        metadata: { managedBy: 'workspace' },
        type: 'user_managed'
      });
      setMemories((current) => [created, ...current.filter((memory) => memory.id !== created.id)]);
      setTotal((current) => current + 1);
      setContent('');
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to create memory. Retry the request or check the backend session.'));
    } finally {
      setIsCreating(false);
    }
  };

  const beginEditMemory = (memory: AgentMemory) => {
    setEditingMemoryId(memory.id);
    setEditingContent(memory.content);
    setEditingImportance(String(normalizeImportance(memory.importance)));
    setError(null);
  };

  const saveMemory = async () => {
    if (editingMemoryId === null) {
      return;
    }
    const trimmedContent = editingContent.trim();
    if (trimmedContent === '') {
      return;
    }

    const parsedImportance = Number.parseInt(editingImportance, 10);
    const importance = Number.isFinite(parsedImportance) ? parsedImportance : 3;
    setSavingMemoryId(editingMemoryId);
    setError(null);

    try {
      const updated = await memoriesApi.updateMemory(editingMemoryId, {
        content: trimmedContent,
        importance
      });
      setMemories((current) => current.map((memory) => (memory.id === updated.id ? updated : memory)));
      setEditingMemoryId(null);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to update memory. Retry the request or check the backend session.'));
    } finally {
      setSavingMemoryId(null);
    }
  };

  const deleteMemory = async (memoryId: string) => {
    setDeletingMemoryId(memoryId);
    setError(null);

    try {
      await memoriesApi.deleteMemory(memoryId);
      setMemories((current) => current.filter((memory) => memory.id !== memoryId));
      setTotal((current) => Math.max(0, current - 1));
      if (editingMemoryId === memoryId) {
        setEditingMemoryId(null);
      }
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to delete memory. Retry the request or check the backend session.'));
    } finally {
      setDeletingMemoryId(null);
    }
  };

  const searchMemories = async () => {
    const trimmedQuery = query.trim();
    if (trimmedQuery === '') {
      return;
    }

    setIsSearching(true);
    setError(null);

    try {
      const parsedLimit = Number.parseInt(resultLimit, 10);
      const limit = Number.isFinite(parsedLimit) && parsedLimit > 0 ? parsedLimit : 10;
      const response = await memoriesApi.searchMemories({
        agentId: agentId.trim() || undefined,
        limit,
        query: trimmedQuery,
        topK: limit,
        type: memoryType || undefined
      });
      setMemories(response.data);
      setTotal(response.total);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to search memories. Retry the request or check the backend session.'));
    } finally {
      setIsSearching(false);
    }
  };

  const exportMemories = async () => {
    setIsExporting(true);
    setError(null);

    try {
      const parsedLimit = Number.parseInt(resultLimit, 10);
      const limit = Number.isFinite(parsedLimit) && parsedLimit > 0 ? parsedLimit : 10;
      const response = await memoriesApi.exportMemories({
        agentId: agentId.trim() || undefined,
        limit,
        query: query.trim() || undefined,
        type: memoryType || undefined
      });
      const exportBlob = new Blob([JSON.stringify(memoryExportPayload(response.data), null, 2)], {
        type: 'application/json;charset=utf-8'
      });
      setExportUrl(URL.createObjectURL(exportBlob));
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to export memories. Retry the request or check the backend session.'));
    } finally {
      setIsExporting(false);
    }
  };

  const importMemories = async (file: File | undefined) => {
    if (!file) {
      return;
    }

    setIsImporting(true);
    setError(null);

    try {
      const parsed = JSON.parse(await readFileAsText(file));
      const entries = Array.isArray(parsed) ? parsed : [parsed];
      const payloads = entries.map(importableMemoryPayload).filter((payload) => payload !== null);
      if (payloads.length === 0) {
        throw new Error('The import file does not contain any valid memories.');
      }

      const imported = await memoriesApi.importMemories(payloads);
      setMemories((current) => [...imported, ...current.filter((memory) => !imported.some((item) => item.id === memory.id))]);
      setTotal((current) => current + imported.length);
    } catch (caughtError) {
      setError(errorMessage(caughtError, 'Unable to import memories. Check the JSON file and try again.'));
    } finally {
      setIsImporting(false);
    }
  };

  return (
    <section className="mx-auto min-w-0 max-w-6xl space-y-6">
      <header className="min-w-0 space-y-2">
        <p className="text-xs font-semibold uppercase tracking-wide text-[#6d6658]">Agent context</p>
        <h1 className="font-heading text-3xl font-semibold text-[#181611]">Agent Memories</h1>
        <p className="max-w-3xl text-sm leading-6 text-[#625b4f]">
          Search and add user-managed memories that agent runs can retrieve as durable workspace context.
        </p>
      </header>

      {error ? (
        <p className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800" role="alert">
          {error}
        </p>
      ) : null}

      <div className="grid min-w-0 gap-5 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <section className="min-w-0 rounded-lg border border-[#d7d2c4] bg-[#fbfaf7] p-5">
          <h2 className="text-base font-semibold">Create memory</h2>
          <form
            className="mt-4 min-w-0 space-y-4"
            onSubmit={(event) => {
              event.preventDefault();
              void createMemory();
            }}
          >
            <label className="block text-sm font-medium">
              Optional agent ID
              <input
                className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
                onChange={(event) => setAgentId(event.target.value)}
                placeholder="agent_..."
                type="text"
                value={agentId}
              />
            </label>
            <label className="block text-sm font-medium">
              Memory content
              <textarea
                className="mt-2 min-h-32 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm leading-6"
                onChange={(event) => setContent(event.target.value)}
                value={content}
              />
            </label>
            <label className="block text-sm font-medium">
              Memory importance
              <select
                className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
                onChange={(event) => setCreateImportance(event.target.value)}
                value={createImportance}
              >
                <option value="1">1 star</option>
                <option value="2">2 stars</option>
                <option value="3">3 stars</option>
                <option value="4">4 stars</option>
                <option value="5">5 stars</option>
              </select>
            </label>
            <button
              className="min-h-11 rounded-lg bg-[#181611] px-4 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
              disabled={isCreating || content.trim() === ''}
              type="submit"
            >
              {isCreating ? 'Creating memory...' : 'Create memory'}
            </button>
          </form>
        </section>

        <section className="min-w-0 rounded-lg border border-[#d7d2c4] bg-white p-5">
          <h2 className="text-base font-semibold">Search memories</h2>
          <form
            className="mt-4 grid min-w-0 gap-3 sm:grid-cols-[minmax(0,1fr)_9rem_8rem_auto] sm:items-end"
            onSubmit={(event) => {
              event.preventDefault();
              void searchMemories();
            }}
          >
            <label className="flex-1 text-sm font-medium">
              Search query
              <input
                className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
                onChange={(event) => setQuery(event.target.value)}
                type="search"
                value={query}
              />
            </label>
            <label className="text-sm font-medium">
              Memory type
              <select
                className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
                onChange={(event) => setMemoryType(event.target.value)}
                value={memoryType}
              >
                <option value="">All types</option>
                <option value="short_term">Short term</option>
                <option value="long_term">Long term</option>
                <option value="user_managed">User managed</option>
              </select>
            </label>
            <label className="text-sm font-medium">
              Result limit
              <input
                className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
                min="1"
                onChange={(event) => setResultLimit(event.target.value)}
                type="number"
                value={resultLimit}
              />
            </label>
            <button
              className="rounded-lg border border-[#181611] px-4 py-2 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-50"
              disabled={isSearching || query.trim() === ''}
              type="submit"
            >
              {isSearching ? 'Searching...' : 'Search memories'}
            </button>
          </form>

          <section aria-label="Memory results" className="mt-5 min-w-0 space-y-3">
            <div className="flex min-w-0 flex-wrap items-center justify-between gap-3">
              <p className="text-sm text-[#625b4f]">{total === 1 ? '1 memory' : `${total} memories`}</p>
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <button
                  className="min-w-0 max-w-full break-words rounded-lg border border-[#d7d2c4] px-3 py-2 text-left text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-50 [overflow-wrap:anywhere]"
                  disabled={isExporting || memories.length === 0}
                  onClick={() => void exportMemories()}
                  type="button"
                >
                  {isExporting ? 'Exporting...' : 'Export memories'}
                </button>
                {exportUrl ? (
                  <a
                    className="min-w-0 max-w-full break-words rounded-lg bg-[#181611] px-3 py-2 text-sm font-semibold text-white [overflow-wrap:anywhere]"
                    download="agent-memories.json"
                    href={exportUrl}
                  >
                    Download memory export
                  </a>
                ) : null}
                <label className="min-w-0 max-w-full break-words rounded-lg border border-[#d7d2c4] px-3 py-2 text-sm font-semibold [overflow-wrap:anywhere]">
                  {isImporting ? 'Importing...' : 'Import memories JSON'}
                  <input
                    accept="application/json,.json"
                    className="sr-only"
                    disabled={isImporting}
                    onChange={(event) => void importMemories(event.target.files?.[0])}
                    type="file"
                  />
                </label>
              </div>
            </div>
            {memories.length === 0 ? <p className="text-sm text-[#625b4f]">No memories to show yet.</p> : null}
            {memories.map((memory) => (
              <article className="min-w-0 max-w-full rounded-lg border border-[#e4dfd2] bg-[#fbfaf7] p-4" key={memory.id}>
                {editingMemoryId === memory.id ? (
                  <div className="min-w-0 space-y-3">
                    <label className="block text-sm font-medium">
                      Edit memory content
                      <textarea
                        className="mt-2 min-h-24 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm leading-6"
                        onChange={(event) => setEditingContent(event.target.value)}
                        value={editingContent}
                      />
                    </label>
                    <label className="block text-sm font-medium">
                      Edit memory importance
                      <select
                        className="mt-2 w-full rounded-lg border border-[#d7d2c4] bg-white px-3 py-2 text-sm"
                        onChange={(event) => setEditingImportance(event.target.value)}
                        value={editingImportance}
                      >
                        <option value="1">1 star</option>
                        <option value="2">2 stars</option>
                        <option value="3">3 stars</option>
                        <option value="4">4 stars</option>
                        <option value="5">5 stars</option>
                      </select>
                    </label>
                    <div className="flex min-w-0 flex-wrap gap-2">
                      <button
                        className="rounded-lg bg-[#181611] px-3 py-2 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50"
                        disabled={savingMemoryId === memory.id || editingContent.trim() === ''}
                        onClick={() => void saveMemory()}
                        type="button"
                      >
                        {savingMemoryId === memory.id ? 'Saving...' : 'Save memory'}
                      </button>
                      <button
                        className="rounded-lg border border-[#d7d2c4] px-3 py-2 text-sm font-semibold"
                        onClick={() => setEditingMemoryId(null)}
                        type="button"
                      >
                        Cancel
                      </button>
                    </div>
                  </div>
                ) : (
                  <>
                    <div className="flex min-w-0 flex-wrap items-center gap-2 text-xs font-semibold uppercase text-[#6d6658]">
                      <span className="min-w-0 break-words [overflow-wrap:anywhere]">{memoryTypeLabel(memory.type)}</span>
                      <ImportanceBadge importance={memory.importance} />
                      {memory.agentId ? <span className="min-w-0 break-words [overflow-wrap:anywhere]">Agent: {memory.agentId}</span> : null}
                    </div>
                    <p className="mt-2 min-w-0 break-words text-sm leading-6 text-[#181611] [overflow-wrap:anywhere]">{memory.content}</p>
                    {memory.metadata && Object.keys(memory.metadata).length > 0 ? (
                      <p className="mt-2 min-w-0 break-words text-xs text-[#625b4f] [overflow-wrap:anywhere]">Metadata: {formatMetadata(memory.metadata)}</p>
                    ) : memory.createdAt ? (
                      <p className="mt-2 min-w-0 break-words text-xs text-[#625b4f] [overflow-wrap:anywhere]">Created: {memory.createdAt}</p>
                    ) : null}
                    <div className="mt-3 flex min-w-0 flex-wrap gap-2">
                      <button
                        className="rounded-lg border border-[#d7d2c4] px-3 py-2 text-sm font-semibold"
                        onClick={() => beginEditMemory(memory)}
                        type="button"
                      >
                        Edit memory
                      </button>
                      <button
                        className="rounded-lg border border-red-200 px-3 py-2 text-sm font-semibold text-red-700 disabled:cursor-not-allowed disabled:opacity-50"
                        disabled={deletingMemoryId === memory.id}
                        onClick={() => void deleteMemory(memory.id)}
                        type="button"
                      >
                        {deletingMemoryId === memory.id ? 'Deleting...' : 'Delete memory'}
                      </button>
                    </div>
                  </>
                )}
              </article>
            ))}
          </section>
        </section>
      </div>
    </section>
  );
}
