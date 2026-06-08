import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const getRelayPricingSettings = vi.fn();
const updateRelayPricingSettings = vi.fn();
const getUsageLimitSettings = vi.fn();
const updateUsageLimitSettings = vi.fn();
const listUsageLogs = vi.fn();

vi.mock('../../features/admin/api', () => ({
  createAdminApi: () => ({
    getRelayPricingSettings,
    updateRelayPricingSettings,
    getUsageLimitSettings,
    updateUsageLimitSettings,
    listUsageLogs,
  }),
}));

import { AdminSettingsPage } from './AdminSettingsPage';

describe('AdminSettingsPage', () => {
  beforeEach(() => {
    getRelayPricingSettings.mockReset();
    updateRelayPricingSettings.mockReset();
    getUsageLimitSettings.mockReset();
    updateUsageLimitSettings.mockReset();
    listUsageLogs.mockReset();
    listUsageLogs.mockResolvedValue({ data: [], total: 0 });
  });

  it('loads and saves relay pricing multiplier settings', async () => {
    getRelayPricingSettings.mockResolvedValue({
      modelMultipliers: { 'gpt-4o': 1.5 },
      groupMultipliers: { vip: 0.8 },
    });
    getUsageLimitSettings.mockResolvedValue([]);
    updateRelayPricingSettings.mockResolvedValue({
      modelMultipliers: { 'gpt-4o': 2 },
      groupMultipliers: { vip: 0.75 },
    });

    render(<AdminSettingsPage />);

    expect(await screen.findByRole('heading', { name: 'Settings' })).toBeInTheDocument();
    fireEvent.change(await screen.findByLabelText('Model multipliers JSON'), {
      target: { value: '{ "gpt-4o": 2 }' },
    });
    fireEvent.change(screen.getByLabelText('Group multipliers JSON'), {
      target: { value: '{ "vip": 0.75 }' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save Settings' }));

    await waitFor(() => expect(updateRelayPricingSettings).toHaveBeenCalledWith({
      modelMultipliers: { 'gpt-4o': 2 },
      groupMultipliers: { vip: 0.75 },
    }));
    expect(await screen.findByText('Settings saved.')).toBeInTheDocument();
  });

  it('loads and saves usage limit settings', async () => {
    getRelayPricingSettings.mockResolvedValue({
      modelMultipliers: {},
      groupMultipliers: {},
    });
    getUsageLimitSettings.mockResolvedValue([
      {
        organizationId: 'org_1',
        quotaMode: 'organization',
        maxConcurrentRequests: 10,
        windowSeconds: 60,
        maxTokensPerWindow: 1000,
        maxTokensPerRequest: 250,
      },
    ]);
    updateUsageLimitSettings.mockResolvedValue({
      scopeType: 'user',
      scopeId: 'user_1',
      limitType: 'tokens',
      period: 'hour',
      limitValue: 300,
      enabled: true,
    });

    render(<AdminSettingsPage />);

    expect(await screen.findByText('organization org_1')).toBeInTheDocument();
    expect(screen.getByText('Mode: organization')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Scope type'), {
      target: { value: 'user' },
    });
    fireEvent.change(screen.getByLabelText('Scope ID'), {
      target: { value: 'user_1' },
    });
    fireEvent.change(screen.getByLabelText('Limit type'), {
      target: { value: 'tokens' },
    });
    fireEvent.change(screen.getByLabelText('Period'), {
      target: { value: 'hour' },
    });
    fireEvent.change(screen.getByLabelText('Limit value'), {
      target: { value: '300' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save Usage Limit' }));

    await waitFor(() => expect(updateUsageLimitSettings).toHaveBeenCalledWith({
      id: undefined,
      scopeType: 'user',
      scopeId: 'user_1',
      limitType: 'tokens',
      period: 'hour',
      limitValue: 300,
      enabled: true,
      userId: 'user_1',
      quotaMode: 'user',
      maxConcurrentRequests: 0,
      windowSeconds: 3600,
      maxTokensPerWindow: 300,
      maxTokensPerRequest: 0,
    }));
    expect(await screen.findByText('Usage limit saved.')).toBeInTheDocument();
  });

  it('shows usage-limit enforcement and recovery signals from usage logs', async () => {
    getRelayPricingSettings.mockResolvedValue({
      modelMultipliers: {},
      groupMultipliers: {},
    });
    getUsageLimitSettings.mockResolvedValue([
      {
        organizationId: 'org_1',
        quotaMode: 'organization',
        maxConcurrentRequests: 0,
        windowSeconds: 60,
        maxTokensPerWindow: 1000,
        maxTokensPerRequest: 250,
      },
    ]);
    listUsageLogs
      .mockResolvedValueOnce({
        data: [
          {
            id: 'usage_limited_1',
            organizationId: 'org_1',
            userId: 'user_1',
            requestId: 'req_limited',
            model: 'gpt-4o',
            status: 'error',
            statusCode: 429,
            errorCode: 'relay_rate_limited',
            cost: 0,
            channelCost: 0,
            promptTokens: 0,
            completionTokens: 0,
            totalTokens: 0,
            createdAt: '2026-06-08T08:00:00Z',
          },
        ],
        total: 1,
      })
      .mockResolvedValueOnce({
        data: [
          {
            id: 'usage_recovered_1',
            organizationId: 'org_1',
            userId: 'user_1',
            requestId: 'req_recovered',
            model: 'gpt-4o',
            status: 'success',
            statusCode: 200,
            cost: 0.01,
            channelCost: 0.005,
            promptTokens: 10,
            completionTokens: 20,
            totalTokens: 30,
            createdAt: '2026-06-08T08:05:00Z',
          },
        ],
        total: 1,
      });

    render(<AdminSettingsPage />);

    expect(await screen.findByText('Recovered')).toBeInTheDocument();
    expect(screen.getByText('1 recent hit - relay_rate_limited')).toBeInTheDocument();
    expect(screen.getByText('Recovery: req_recovered')).toBeInTheDocument();
    expect(listUsageLogs).toHaveBeenNthCalledWith(1, expect.objectContaining({ organizationID: 'org_1', status: 'error', limit: 50 }));
    expect(listUsageLogs).toHaveBeenNthCalledWith(2, expect.objectContaining({ organizationID: 'org_1', status: 'success', limit: 1 }));
  });

  it('selects an existing usage limit for safe editing before saving', async () => {
    getRelayPricingSettings.mockResolvedValue({
      modelMultipliers: {},
      groupMultipliers: {},
    });
    getUsageLimitSettings.mockResolvedValue([
      {
        id: 'limit_org_workspace_day',
        scopeType: 'organization',
        scopeId: 'org_1',
        limitType: 'workspace_chat',
        period: 'day',
        limitValue: 5000,
        enabled: true,
      },
      {
        id: 'limit_user_images_hour',
        scopeType: 'user',
        scopeId: 'user_1',
        limitType: 'image_generation',
        period: 'hour',
        limitValue: 25,
        enabled: false,
        maxConcurrentRequests: 4,
        maxTokensPerWindow: 100,
        maxTokensPerRequest: 20,
      },
    ]);
    updateUsageLimitSettings.mockResolvedValue({
      id: 'limit_user_images_hour',
      scopeType: 'user',
      scopeId: 'user_1',
      limitType: 'image_generation',
      period: 'hour',
      limitValue: 30,
      enabled: true,
    });

    render(<AdminSettingsPage />);

    fireEvent.click(await screen.findByRole('button', { name: 'Edit user user_1 image_generation hour' }));

    expect(screen.getByLabelText('Scope type')).toHaveValue('user');
    expect(screen.getByLabelText('Scope ID')).toHaveValue('user_1');
    expect(screen.getByLabelText('Limit type')).toHaveValue('image_generation');
    expect(screen.getByLabelText('Period')).toHaveValue('hour');
    expect(screen.getByLabelText('Limit value')).toHaveValue(25);
    expect(screen.getByLabelText('Enabled')).not.toBeChecked();

    fireEvent.change(screen.getByLabelText('Limit value'), {
      target: { value: '30' },
    });
    fireEvent.click(screen.getByLabelText('Enabled'));
    fireEvent.click(screen.getByRole('button', { name: 'Save Usage Limit' }));

    await waitFor(() => expect(updateUsageLimitSettings).toHaveBeenCalledWith({
      id: 'limit_user_images_hour',
      scopeType: 'user',
      scopeId: 'user_1',
      limitType: 'image_generation',
      period: 'hour',
      limitValue: 30,
      enabled: true,
      userId: 'user_1',
      quotaMode: 'user',
      maxConcurrentRequests: 4,
      windowSeconds: 3600,
      maxTokensPerWindow: 30,
      maxTokensPerRequest: 20,
    }));
    expect(screen.getByDisplayValue('user_1')).toBeInTheDocument();
  });
});
