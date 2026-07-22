import { cleanup, render, screen } from '@testing-library/react';
import { RouterProvider } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { routerFuture } from './routerFuture';

vi.mock('../app/providers', () => ({
  useAppContext: () => ({
    authState: {
      status: 'authenticated',
      user: { email: 'admin@example.com', id: 'admin_1', role: 'admin' },
      preferences: { defaultMode: 'chat', onboardingCompleted: false },
    },
    bootstrapAuth: () => Promise.resolve(),
    updatePreferences: (preferences: unknown) => Promise.resolve(preferences),
  }),
}));

vi.mock('../routes/marketing/HomePage', () => ({
  HomePage: () => <main data-testid="route-page">HomePage</main>,
}));
vi.mock('../routes/marketing/LoginPage', () => ({
  LoginPage: () => <main data-testid="route-page">LoginPage</main>,
}));
vi.mock('../routes/marketing/RegisterPage', () => ({
  RegisterPage: () => <main data-testid="route-page">RegisterPage</main>,
}));
vi.mock('../routes/marketing/PricingPage', () => ({
  PricingPage: () => <main data-testid="route-page">PricingPage</main>,
}));
vi.mock('../routes/marketing/DownloadPage', () => ({
  DownloadPage: () => <main data-testid="route-page">DownloadPage</main>,
}));

vi.mock('../routes/workspace/OnboardingPage', () => ({
  OnboardingPage: () => <main data-testid="route-page">OnboardingPage</main>,
}));
vi.mock('../routes/workspace/ChatPage', () => ({
  ChatPage: () => <main data-testid="route-page">ChatPage</main>,
}));
vi.mock('../routes/workspace/KnowledgePage', () => ({
  KnowledgePage: () => <main data-testid="route-page">KnowledgePage</main>,
}));
vi.mock('../routes/workspace/AgentsPage', () => ({
  AgentsPage: () => <main data-testid="route-page">AgentsPage</main>,
}));
vi.mock('../routes/workspace/McpServersPage', () => ({
  McpServersPage: () => <main data-testid="route-page">McpServersPage</main>,
}));
vi.mock('../routes/workspace/AgentMemoriesPage', () => ({
  AgentMemoriesPage: () => <main data-testid="route-page">AgentMemoriesPage</main>,
}));
vi.mock('../routes/workspace/AgentPlanStepsPage', () => ({
  AgentPlanStepsPage: () => <main data-testid="route-page">AgentPlanStepsPage</main>,
}));
vi.mock('../routes/workspace/SoloPage', () => ({
  SoloPage: () => <main data-testid="route-page">SoloPage</main>,
}));
vi.mock('../routes/workspace/WorkflowsPage', () => ({
  WorkflowsPage: () => <main data-testid="route-page">WorkflowsPage</main>,
}));
vi.mock('../routes/workspace/ScheduledTasksPage', () => ({
  ScheduledTasksPage: () => <main data-testid="route-page">ScheduledTasksPage</main>,
}));
vi.mock('../routes/workspace/PublishingChannelsPage', () => ({
  PublishingChannelsPage: () => <main data-testid="route-page">PublishingChannelsPage</main>,
}));
vi.mock('../routes/workspace/SettingsPage', () => ({
  SettingsPage: () => <main data-testid="route-page">SettingsPage</main>,
}));

vi.mock('../routes/marketplace/MarketplaceHomePage', () => ({
  MarketplaceHomePage: () => <main data-testid="route-page">MarketplaceHomePage</main>,
}));
vi.mock('../routes/marketplace/MarketplaceAgentDetailPage', () => ({
  MarketplaceAgentDetailPage: () => <main data-testid="route-page">MarketplaceAgentDetailPage</main>,
}));
vi.mock('../routes/marketplace/MarketplacePublishPage', () => ({
  MarketplacePublishPage: () => <main data-testid="route-page">MarketplacePublishPage</main>,
}));
vi.mock('../routes/marketplace/MarketplaceMyAgentsPage', () => ({
  MarketplaceMyAgentsPage: () => <main data-testid="route-page">MarketplaceMyAgentsPage</main>,
}));

vi.mock('../routes/console/ConsoleHomePage', () => ({
  ConsoleHomePage: () => <main data-testid="route-page">ConsoleHomePage</main>,
}));
vi.mock('../routes/console/ModelsPage', () => ({
  ModelsPage: () => <main data-testid="route-page">ModelsPage</main>,
}));
vi.mock('../routes/console/UsagePage', () => ({
  UsagePage: () => <main data-testid="route-page">UsagePage</main>,
}));
vi.mock('../routes/console/BillingPage', () => ({
  BillingPage: () => <main data-testid="route-page">BillingPage</main>,
}));
vi.mock('../routes/console/AccessPage', () => ({
  AccessPage: () => <main data-testid="route-page">AccessPage</main>,
}));
vi.mock('../routes/console/NotificationsPage', () => ({
  NotificationsPage: () => <main data-testid="route-page">NotificationsPage</main>,
}));

vi.mock('../routes/admin/AdminHomePage', () => ({
  AdminHomePage: () => <main data-testid="route-page">AdminHomePage</main>,
}));
vi.mock('../routes/admin/AdminChannelsPage', () => ({
  AdminChannelsPage: () => <main data-testid="route-page">AdminChannelsPage</main>,
}));
vi.mock('../routes/admin/AdminModelsPage', () => ({
  AdminModelsPage: () => <main data-testid="route-page">AdminModelsPage</main>,
}));
vi.mock('../routes/admin/AdminRoutesPage', () => ({
  AdminRoutesPage: () => <main data-testid="route-page">AdminRoutesPage</main>,
}));
vi.mock('../routes/admin/AdminPlansPage', () => ({
  AdminPlansPage: () => <main data-testid="route-page">AdminPlansPage</main>,
}));
vi.mock('../routes/admin/AdminBillingPage', () => ({
  AdminBillingPage: () => <main data-testid="route-page">AdminBillingPage</main>,
}));
vi.mock('../routes/admin/AdminAPITokensPage', () => ({
  AdminAPITokensPage: () => <main data-testid="route-page">AdminAPITokensPage</main>,
}));
vi.mock('../routes/admin/AdminUsageLogsPage', () => ({
  AdminUsageLogsPage: () => <main data-testid="route-page">AdminUsageLogsPage</main>,
}));
vi.mock('../routes/admin/AdminAlertsPage', () => ({
  AdminAlertsPage: () => <main data-testid="route-page">AdminAlertsPage</main>,
}));
vi.mock('../routes/admin/AdminSettingsPage', () => ({
  AdminSettingsPage: () => <main data-testid="route-page">AdminSettingsPage</main>,
}));
vi.mock('../routes/admin/AdminUsersPage', () => ({
  AdminUsersPage: () => <main data-testid="route-page">AdminUsersPage</main>,
}));
vi.mock('../routes/admin/AdminAuditLogPage', () => ({
  AdminAuditLogPage: () => <main data-testid="route-page">AdminAuditLogPage</main>,
}));
vi.mock('../routes/admin/AdminReviewsPage', () => ({
  AdminReviewsPage: () => <main data-testid="route-page">AdminReviewsPage</main>,
}));

import { appRouteEntries, createAppRouter } from './router';
import { getGeneratedReleaseCapability } from '../features/releaseProjection/releaseProjection';

const expectedPageBySamplePath: Record<string, string> = {
  '/': 'HomePage',
  '/login': 'LoginPage',
  '/register': 'RegisterPage',
  '/pricing': 'PricingPage',
  '/download': 'DownloadPage',
  '/onboarding': 'OnboardingPage',
  '/chat': 'ChatPage',
  '/chat/conversation_1': 'ChatPage',
  '/knowledge': 'KnowledgePage',
  '/knowledge/kb_1': 'KnowledgePage',
  '/agents': 'AgentsPage',
  '/mcp-servers': 'McpServersPage',
  '/memories': 'AgentMemoriesPage',
  '/agent-runs/run_1/plan-steps': 'AgentPlanStepsPage',
  '/solo': 'SoloPage',
  '/solo/new': 'SoloPage',
  '/workflows': 'WorkflowsPage',
  '/scheduled-tasks': 'ScheduledTasksPage',
  '/publishing': 'PublishingChannelsPage',
  '/marketplace': 'MarketplaceHomePage',
  '/marketplace/agents/agent_1': 'MarketplaceAgentDetailPage',
  '/marketplace/publish': 'MarketplacePublishPage',
  '/marketplace/my-agents': 'MarketplaceMyAgentsPage',
  '/settings': 'SettingsPage',
  '/console': 'ConsoleHomePage',
  '/console/models': 'ModelsPage',
  '/console/usage': 'UsagePage',
  '/console/billing': 'BillingPage',
  '/console/access': 'AccessPage',
  '/console/notifications': 'NotificationsPage',
  '/admin': 'AdminHomePage',
  '/admin/channels': 'AdminChannelsPage',
  '/admin/models': 'AdminModelsPage',
  '/admin/routes': 'AdminRoutesPage',
  '/admin/plans': 'AdminPlansPage',
  '/admin/billing': 'AdminBillingPage',
  '/admin/api-tokens': 'AdminAPITokensPage',
  '/admin/usage-logs': 'AdminUsageLogsPage',
  '/admin/alerts': 'AdminAlertsPage',
  '/admin/settings': 'AdminSettingsPage',
  '/admin/users': 'AdminUsersPage',
  '/admin/audit-log': 'AdminAuditLogPage',
  '/admin/reviews': 'AdminReviewsPage',
};

afterEach(() => {
  cleanup();
});

describe('app route surface', () => {
  it('keeps route manifest sample paths unique and explicitly asserted', () => {
    const samplePaths = appRouteEntries.map((entry) => entry.samplePath);

    expect(new Set(samplePaths).size).toBe(samplePaths.length);
    expect(Object.keys(expectedPageBySamplePath).sort()).toEqual([...samplePaths].sort());
  });

  it.each(appRouteEntries)('renders $samplePath through the $area shell', async (entry) => {
    const router = createAppRouter([entry.samplePath]);

    render(<RouterProvider future={routerFuture} router={router} />);

    const generated = entry.capabilityId === undefined ? null : getGeneratedReleaseCapability(entry.capabilityId);
    if (generated?.disposition === 'conditional') {
      expect(await screen.findByRole('status')).toHaveTextContent('currently unavailable');
      expect(screen.queryByTestId('route-page')).not.toBeInTheDocument();
    } else {
      expect(await screen.findByTestId('route-page')).toHaveTextContent(expectedPageBySamplePath[entry.samplePath]);
    }
    if (entry.area === 'workspace') {
      expect(document.querySelector('[data-gsap-scope="workspace"]')).toBeInTheDocument();
    }
    if (entry.area === 'console') {
      expect(document.querySelector('[data-gsap-scope="console"]')).toBeInTheDocument();
    }
    if (entry.area === 'admin') {
      expect(document.querySelector('[data-gsap-scope="admin"]')).toBeInTheDocument();
    }
  });
});
