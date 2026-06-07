import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { routerFuture } from '../../app/routerFuture';

const getAccess = vi.fn();
const listApiTokens = vi.fn();
const listApiTokenUsage = vi.fn();
const createApiToken = vi.fn();
const revokeApiToken = vi.fn();

vi.mock('../../features/console/api', () => ({
  createConsoleApi: () => ({
    createApiToken,
    getAccess,
    listApiTokenUsage,
    listApiTokens,
    revokeApiToken
  })
}));

import { AccessPage } from './AccessPage';

describe('AccessPage', () => {
  afterEach(() => {
    createApiToken.mockReset();
    getAccess.mockReset();
    listApiTokenUsage.mockReset();
    listApiTokens.mockReset();
    revokeApiToken.mockReset();
  });

  it('renders the access page as a scope explanation workbench', async () => {
    getAccess.mockResolvedValue({
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: true,
      onboardingCompleted: true,
      sessionExpiresAt: '2026-04-03T00:00:00Z',
      sessionId: 'session_1',
      userEmail: 'user@example.com',
      userId: 'user_1',
      workspaceId: 'workspace_1'
    });
    listApiTokens.mockResolvedValue([]);

    render(
      <MemoryRouter future={routerFuture}>
        <AccessPage />
      </MemoryRouter>
    );

    expect(await screen.findByText('Current workspace scope')).toBeInTheDocument();
    expect(await screen.findByText('This console reflects the active workspace and current session.')).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Workspace settings' })).toBeInTheDocument();
  });

  it('lists API tokens and creates a new relay key', async () => {
    getAccess.mockResolvedValue({
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: true,
      onboardingCompleted: true,
      sessionExpiresAt: '2026-04-03T00:00:00Z',
      sessionId: 'session_1',
      userEmail: 'user@example.com',
      userId: 'user_1',
      workspaceId: 'workspace_1'
    });
    listApiTokens.mockResolvedValueOnce([
      {
        id: 'tok_1',
        modelLimits: ['gpt-4o'],
        modelLimitsEnabled: true,
        name: 'CI gateway key',
        status: 'active',
        tokenPrefix: 'obv_live_123',
        usedQuota: 0,
        createdAt: '2026-05-30T00:00:00Z'
      }
    ]);
    createApiToken.mockResolvedValue({
      rawToken: 'obv_new_secret',
      token: {
        id: 'tok_2',
        modelLimits: ['gpt-4o-mini'],
        modelLimitsEnabled: true,
        name: 'Browser key',
        status: 'active',
        tokenPrefix: 'obv_new_sec',
        usedQuota: 0,
        createdAt: '2026-05-30T00:00:00Z'
      }
    });

    render(
      <MemoryRouter future={routerFuture}>
        <AccessPage />
      </MemoryRouter>
    );

    expect(await screen.findByText('API tokens')).toBeInTheDocument();
    expect(await screen.findByText('CI gateway key')).toBeInTheDocument();
    expect(screen.getByText('obv_live_123')).toBeInTheDocument();

	    fireEvent.change(screen.getByLabelText('Token name'), { target: { value: 'Browser key' } });
	    fireEvent.change(screen.getByLabelText('Allowed models'), { target: { value: 'gpt-4o-mini' } });
	    fireEvent.change(screen.getByLabelText('Routing group'), { target: { value: 'vip' } });
	    fireEvent.change(screen.getByLabelText('Quota limit'), { target: { value: '25.5' } });
	    fireEvent.change(screen.getByLabelText('Expires at'), { target: { value: '2026-06-30T00:00:00Z' } });
	    fireEvent.click(screen.getByRole('button', { name: 'Create API token' }));

	    expect(await screen.findByText('obv_new_secret')).toBeInTheDocument();
	    expect(createApiToken).toHaveBeenCalledWith({
	      modelLimits: ['gpt-4o-mini'],
	      modelLimitsEnabled: true,
	      name: 'Browser key',
	      quotaLimit: 25.5,
	      expiresAt: '2026-06-30T00:00:00Z',
	      userGroup: 'vip'
	    });
	  });

  it('loads API token usage details on demand', async () => {
    getAccess.mockResolvedValue({
      defaultMode: 'chat',
      modelStrategy: 'balanced',
      networkEnabledHint: true,
      onboardingCompleted: true,
      sessionExpiresAt: '2026-04-03T00:00:00Z',
      sessionId: 'session_1',
      userEmail: 'user@example.com',
      userId: 'user_1',
      workspaceId: 'workspace_1'
    });
    listApiTokens.mockResolvedValueOnce([
      {
        id: 'tok_1',
        modelLimits: ['gpt-4o'],
        modelLimitsEnabled: true,
        name: 'CI gateway key',
        quotaLimit: 10,
        status: 'active',
        tokenPrefix: 'obv_live_123',
        usedQuota: 2.5,
        createdAt: '2026-05-30T00:00:00Z'
      }
    ]);
    listApiTokenUsage.mockResolvedValueOnce([
      {
        id: 'usage_1',
        apiTokenId: 'tok_1',
        apiType: 'chat',
        channelId: 'ch_1',
        completionTokens: 100,
        cost: 0.004,
        createdAt: '2026-05-30T00:00:00Z',
        latencyMs: 42,
        model: 'gpt-4o',
        promptTokens: 1000,
        provider: 'openai',
        requestId: 'req_1',
        status: 'success',
        statusCode: 200,
        totalTokens: 1100
      }
    ]);

    render(
      <MemoryRouter future={routerFuture}>
        <AccessPage />
      </MemoryRouter>
    );

    expect(await screen.findByText('2.5 / 10 quota')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'View usage for CI gateway key' }));

    expect(await screen.findByText('req_1')).toBeInTheDocument();
    expect(screen.getAllByText('gpt-4o')).toHaveLength(2);
    expect(screen.getByText('chat')).toBeInTheDocument();
    expect(screen.getByText('openai / ch_1')).toBeInTheDocument();
    expect(screen.getByText('42 ms')).toBeInTheDocument();
    expect(screen.getByText('$0.004')).toBeInTheDocument();
    expect(listApiTokenUsage).toHaveBeenCalledWith('tok_1');
  });
});
