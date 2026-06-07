import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const createMemory = vi.fn();
const deleteMemory = vi.fn();
const exportMemories = vi.fn();
const importMemories = vi.fn();
const searchMemories = vi.fn();
const updateMemory = vi.fn();

vi.mock('../../features/agents/memoriesApi', () => ({
  createAgentMemoriesApi: () => ({
    createMemory,
    deleteMemory,
    exportMemories,
    importMemories,
    searchMemories,
    updateMemory
  })
}));

import { AgentMemoriesPage } from './AgentMemoriesPage';

describe('AgentMemoriesPage', () => {
  beforeEach(() => {
    createMemory.mockReset();
    deleteMemory.mockReset();
    exportMemories.mockReset();
    importMemories.mockReset();
    searchMemories.mockReset();
    updateMemory.mockReset();
  });

  it('creates user-managed memories, renders search results, and preserves results when errors occur', async () => {
    searchMemories.mockResolvedValueOnce({
      data: [
        {
          agentId: 'agent_1',
          content: 'Prefer concise rollout notes.',
          createdAt: '2026-06-05T08:15:00Z',
          id: 'memory_1',
          importance: 4,
          metadata: { source: 'workflow', topic: 'release' },
          type: 'long_term'
        }
      ],
      total: 1
    });
    createMemory.mockResolvedValueOnce({
      agentId: 'agent_1',
      content: 'Always include migration rollback notes.',
      id: 'memory_2',
      importance: 5,
      type: 'user_managed'
    });
    searchMemories.mockRejectedValueOnce(new Error('Search unavailable'));

    render(<AgentMemoriesPage />);

    expect(screen.getByRole('heading', { name: 'Agent Memories' })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Optional agent ID'), { target: { value: 'agent_1' } });
    fireEvent.change(screen.getByLabelText('Memory type'), { target: { value: 'long_term' } });
    fireEvent.change(screen.getByLabelText('Result limit'), { target: { value: '5' } });
    fireEvent.change(screen.getByLabelText('Search query'), { target: { value: 'rollout' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search memories' }));

    const resultCard = (await screen.findByText('Prefer concise rollout notes.')).closest('article');
    expect(resultCard).not.toBeNull();
    expect(searchMemories).toHaveBeenCalledWith({ agentId: 'agent_1', limit: 5, query: 'rollout', topK: 5, type: 'long_term' });
    expect(within(resultCard as HTMLElement).getByText('Long term')).toBeInTheDocument();
    expect(within(resultCard as HTMLElement).getByText('Agent: agent_1')).toBeInTheDocument();
    expect(within(resultCard as HTMLElement).getByLabelText('Importance 4 of 5')).toBeInTheDocument();
    expect(within(resultCard as HTMLElement).getByText('Metadata: source=workflow, topic=release')).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Memory importance'), { target: { value: '5' } });
    fireEvent.change(screen.getByLabelText('Memory content'), {
      target: { value: 'Always include migration rollback notes.' }
    });
    fireEvent.click(screen.getByRole('button', { name: 'Create memory' }));

    await waitFor(() => {
      expect(createMemory).toHaveBeenCalledWith({
        agentId: 'agent_1',
        content: 'Always include migration rollback notes.',
        importance: 5,
        metadata: { managedBy: 'workspace' },
        type: 'user_managed'
      });
    });

    expect(screen.getByText('Always include migration rollback notes.')).toBeInTheDocument();
    expect(screen.getByLabelText('Memory content')).toHaveValue('');

    fireEvent.change(screen.getByLabelText('Search query'), { target: { value: 'broken' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search memories' }));

    expect(await screen.findByText('Search unavailable')).toBeInTheDocument();
    expect(screen.getByText('Always include migration rollback notes.')).toBeInTheDocument();
    expect(screen.getByText('Prefer concise rollout notes.')).toBeInTheDocument();
  });

  it('edits and deletes existing memories from the results list', async () => {
    searchMemories.mockResolvedValueOnce({
      data: [
        {
          agentId: 'agent_1',
          content: 'Draft migration notes.',
          id: 'memory_1',
          importance: 2,
          type: 'user_managed'
        }
      ],
      total: 1
    });
    updateMemory.mockResolvedValueOnce({
      agentId: 'agent_1',
      content: 'Publish migration notes.',
      id: 'memory_1',
      importance: 5,
      type: 'user_managed'
    });
    deleteMemory.mockResolvedValueOnce(undefined);

    render(<AgentMemoriesPage />);

    fireEvent.change(screen.getByLabelText('Search query'), { target: { value: 'migration' } });
    fireEvent.click(screen.getByRole('button', { name: 'Search memories' }));

    const resultCard = (await screen.findByText('Draft migration notes.')).closest('article');
    expect(resultCard).not.toBeNull();

    fireEvent.click(within(resultCard as HTMLElement).getByRole('button', { name: 'Edit memory' }));
    fireEvent.change(screen.getByLabelText('Edit memory content'), { target: { value: 'Publish migration notes.' } });
    fireEvent.change(screen.getByLabelText('Edit memory importance'), { target: { value: '5' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save memory' }));

    await waitFor(() => {
      expect(updateMemory).toHaveBeenCalledWith('memory_1', {
        content: 'Publish migration notes.',
        importance: 5
      });
    });
    expect(await screen.findByText('Publish migration notes.')).toBeInTheDocument();
    expect(screen.getByLabelText('Importance 5 of 5')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Delete memory' }));

    await waitFor(() => {
      expect(deleteMemory).toHaveBeenCalledWith('memory_1');
    });
    expect(screen.queryByText('Publish migration notes.')).not.toBeInTheDocument();
    expect(screen.getByText('0 memories')).toBeInTheDocument();
  });

  it('exports current memories and imports user-managed memories from JSON', async () => {
    const createObjectURL = vi.fn(() => 'blob:agent-memories-export');
    const originalCreateObjectURL = URL.createObjectURL;
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: createObjectURL
    });
    searchMemories.mockResolvedValueOnce({
      data: [
        {
          agentId: 'agent_1',
          content: 'Prefer concise rollout notes.',
          id: 'memory_1',
          importance: 4,
          metadata: { topic: 'release' },
          type: 'user_managed'
        }
      ],
      total: 1
    });
    exportMemories.mockResolvedValueOnce({
      data: [
        {
          agentId: 'agent_1',
          content: 'Prefer concise rollout notes.',
          id: 'memory_1',
          importance: 4,
          metadata: { topic: 'release' },
          type: 'user_managed'
        }
      ],
      total: 1
    });
    importMemories.mockResolvedValueOnce([
      {
        agentId: 'agent_2',
        content: 'Imported memory one.',
        id: 'memory_import_1',
        importance: 5,
        metadata: { imported: true },
        type: 'user_managed'
      },
      {
        content: 'Imported memory two.',
        id: 'memory_import_2',
        importance: 3,
        type: 'user_managed'
      }
    ]);

    try {
      render(<AgentMemoriesPage />);

      fireEvent.change(screen.getByLabelText('Search query'), { target: { value: 'rollout' } });
      fireEvent.click(screen.getByRole('button', { name: 'Search memories' }));

      await screen.findByText('Prefer concise rollout notes.');
      fireEvent.click(screen.getByRole('button', { name: 'Export memories' }));

      await waitFor(() => {
        expect(exportMemories).toHaveBeenCalledWith({ agentId: undefined, limit: 10, query: 'rollout', type: undefined });
      });
      expect(createObjectURL).toHaveBeenCalledTimes(1);
      expect(await screen.findByRole('link', { name: 'Download memory export' })).toHaveAttribute(
        'href',
        'blob:agent-memories-export'
      );

      const importFile = new File(
        [
          JSON.stringify([
            {
              agentId: 'agent_2',
              content: 'Imported memory one.',
              importance: 5,
              metadata: { imported: true },
              type: 'user_managed'
            },
            {
              content: 'Imported memory two.',
              type: 'user_managed'
            }
          ])
        ],
        'agent-memories.json',
        { type: 'application/json' }
      );
      fireEvent.change(screen.getByLabelText('Import memories JSON'), { target: { files: [importFile] } });

      await waitFor(() => {
        expect(importMemories).toHaveBeenCalledTimes(1);
      });
      expect(importMemories).toHaveBeenCalledWith([
        {
          agentId: 'agent_2',
          content: 'Imported memory one.',
          importance: 5,
          metadata: { imported: true },
          type: 'user_managed'
        },
        {
          content: 'Imported memory two.',
          importance: 3,
          metadata: { importedBy: 'workspace_import' },
          type: 'user_managed'
        }
      ]);
      expect(await screen.findByText('Imported memory one.')).toBeInTheDocument();
      expect(screen.getByText('Imported memory two.')).toBeInTheDocument();
      expect(screen.getByText('3 memories')).toBeInTheDocument();
    } finally {
      Object.defineProperty(URL, 'createObjectURL', {
        configurable: true,
        value: originalCreateObjectURL
      });
    }
  });
});
