import {
  createBrowserRouter,
  createMemoryRouter,
  type RouteObject
} from 'react-router-dom';

import { routerFuture } from './routerFuture';
import { ProtectedRoute } from '../features/auth/ProtectedRoute';
import { AdminRoute } from '../features/auth/AdminRoute';
import { ConsoleLayout } from '../features/layouts/ConsoleLayout';
import { MarketingLayout } from '../features/layouts/MarketingLayout';
import { WorkspaceLayout } from '../features/layouts/WorkspaceLayout';
import { AdminLayout } from '../features/layouts/AdminLayout';
import { AccessPage } from '../routes/console/AccessPage';
import { BillingPage } from '../routes/console/BillingPage';
import { ConsoleHomePage } from '../routes/console/ConsoleHomePage';
import { ModelsPage } from '../routes/console/ModelsPage';
import { NotificationsPage } from '../routes/console/NotificationsPage';
import { UsagePage } from '../routes/console/UsagePage';
import { DownloadPage } from '../routes/marketing/DownloadPage';
import { HomePage } from '../routes/marketing/HomePage';
import { LoginPage } from '../routes/marketing/LoginPage';
import { PricingPage } from '../routes/marketing/PricingPage';
import { RegisterPage } from '../routes/marketing/RegisterPage';
import { AgentsPage } from '../routes/workspace/AgentsPage';
import { AgentMemoriesPage } from '../routes/workspace/AgentMemoriesPage';
import { AgentPlanStepsPage } from '../routes/workspace/AgentPlanStepsPage';
import { ChatPage } from '../routes/workspace/ChatPage';
import { KnowledgePage } from '../routes/workspace/KnowledgePage';
import { McpServersPage } from '../routes/workspace/McpServersPage';
import { OnboardingPage } from '../routes/workspace/OnboardingPage';
import { PublishingChannelsPage } from '../routes/workspace/PublishingChannelsPage';
import { ScheduledTasksPage } from '../routes/workspace/ScheduledTasksPage';
import { SettingsPage } from '../routes/workspace/SettingsPage';
import { SoloPage } from '../routes/workspace/SoloPage';
import { WorkflowsPage } from '../routes/workspace/WorkflowsPage';
import { MarketplaceAgentDetailPage } from '../routes/marketplace/MarketplaceAgentDetailPage';
import { MarketplaceHomePage } from '../routes/marketplace/MarketplaceHomePage';
import { MarketplaceMyAgentsPage } from '../routes/marketplace/MarketplaceMyAgentsPage';
import { MarketplacePublishPage } from '../routes/marketplace/MarketplacePublishPage';
import { AdminHomePage } from '../routes/admin/AdminHomePage';
import { AdminUsersPage } from '../routes/admin/AdminUsersPage';
import { AdminChannelsPage } from '../routes/admin/AdminChannelsPage';
import { AdminModelsPage } from '../routes/admin/AdminModelsPage';
import { AdminRoutesPage } from '../routes/admin/AdminRoutesPage';
import { AdminPlansPage } from '../routes/admin/AdminPlansPage';
import { AdminBillingPage } from '../routes/admin/AdminBillingPage';
import { AdminAuditLogPage } from '../routes/admin/AdminAuditLogPage';
import { AdminReviewsPage } from '../routes/admin/AdminReviewsPage';
import { AdminSettingsPage } from '../routes/admin/AdminSettingsPage';
import { AdminUsageLogsPage } from '../routes/admin/AdminUsageLogsPage';
import { AdminAPITokensPage } from '../routes/admin/AdminAPITokensPage';
import { AdminAlertsPage } from '../routes/admin/AdminAlertsPage';

type AppRouteArea = 'marketing' | 'workspace' | 'console' | 'admin';

type AppRouteEntry = {
  area: AppRouteArea;
  element: RouteObject['element'];
  label: string;
  path?: string;
  index?: boolean;
  samplePath: string;
};

export const marketingRouteEntries: AppRouteEntry[] = [
  { area: 'marketing', path: '/', element: <HomePage />, label: 'Home', samplePath: '/' },
  { area: 'marketing', path: '/login', element: <LoginPage />, label: 'Login', samplePath: '/login' },
  { area: 'marketing', path: '/register', element: <RegisterPage />, label: 'Register', samplePath: '/register' },
  { area: 'marketing', path: '/pricing', element: <PricingPage />, label: 'Pricing', samplePath: '/pricing' },
  { area: 'marketing', path: '/download', element: <DownloadPage />, label: 'Download', samplePath: '/download' },
];

export const workspaceRouteEntries: AppRouteEntry[] = [
  { area: 'workspace', path: '/onboarding', element: <OnboardingPage />, label: 'Onboarding', samplePath: '/onboarding' },
  { area: 'workspace', path: '/chat', element: <ChatPage />, label: 'Chat', samplePath: '/chat' },
  { area: 'workspace', path: '/chat/:conversationId', element: <ChatPage />, label: 'Chat conversation detail', samplePath: '/chat/conversation_1' },
  { area: 'workspace', path: '/knowledge', element: <KnowledgePage />, label: 'Knowledge', samplePath: '/knowledge' },
  { area: 'workspace', path: '/knowledge/:knowledgeBaseId', element: <KnowledgePage />, label: 'Knowledge base detail', samplePath: '/knowledge/kb_1' },
  { area: 'workspace', path: '/agents', element: <AgentsPage />, label: 'Agents', samplePath: '/agents' },
  { area: 'workspace', path: '/mcp-servers', element: <McpServersPage />, label: 'MCP Servers', samplePath: '/mcp-servers' },
  { area: 'workspace', path: '/memories', element: <AgentMemoriesPage />, label: 'Agent Memories', samplePath: '/memories' },
  { area: 'workspace', path: '/agent-runs/:runId/plan-steps', element: <AgentPlanStepsPage />, label: 'Agent Plan Steps', samplePath: '/agent-runs/run_1/plan-steps' },
  { area: 'workspace', path: '/solo', element: <SoloPage />, label: 'SOLO', samplePath: '/solo' },
  { area: 'workspace', path: '/solo/new', element: <SoloPage />, label: 'SOLO new run', samplePath: '/solo/new' },
  { area: 'workspace', path: '/workflows', element: <WorkflowsPage />, label: 'Workflows', samplePath: '/workflows' },
  { area: 'workspace', path: '/scheduled-tasks', element: <ScheduledTasksPage />, label: 'Scheduled Tasks', samplePath: '/scheduled-tasks' },
  { area: 'workspace', path: '/publishing', element: <PublishingChannelsPage />, label: 'Publishing Channels', samplePath: '/publishing' },
  { area: 'workspace', path: '/marketplace', element: <MarketplaceHomePage />, label: 'Agent Marketplace', samplePath: '/marketplace' },
  { area: 'workspace', path: '/marketplace/agents/:agentId', element: <MarketplaceAgentDetailPage />, label: 'Marketplace agent detail', samplePath: '/marketplace/agents/agent_1' },
  { area: 'workspace', path: '/marketplace/publish', element: <MarketplacePublishPage />, label: 'Publish Agent', samplePath: '/marketplace/publish' },
  { area: 'workspace', path: '/marketplace/my-agents', element: <MarketplaceMyAgentsPage />, label: 'My Agents', samplePath: '/marketplace/my-agents' },
  { area: 'workspace', path: '/settings', element: <SettingsPage />, label: 'Settings', samplePath: '/settings' },
];

export const consoleRouteEntries: AppRouteEntry[] = [
  { area: 'console', index: true, element: <ConsoleHomePage />, label: 'Console Home', samplePath: '/console' },
  { area: 'console', path: 'models', element: <ModelsPage />, label: 'Models', samplePath: '/console/models' },
  { area: 'console', path: 'usage', element: <UsagePage />, label: 'Usage', samplePath: '/console/usage' },
  { area: 'console', path: 'billing', element: <BillingPage />, label: 'Billing', samplePath: '/console/billing' },
  { area: 'console', path: 'access', element: <AccessPage />, label: 'Access', samplePath: '/console/access' },
  { area: 'console', path: 'notifications', element: <NotificationsPage />, label: 'Notifications', samplePath: '/console/notifications' },
];

export const adminRouteEntries: AppRouteEntry[] = [
  { area: 'admin', index: true, element: <AdminHomePage />, label: 'Admin dashboard', samplePath: '/admin' },
  { area: 'admin', path: 'channels', element: <AdminChannelsPage />, label: 'Channels', samplePath: '/admin/channels' },
  { area: 'admin', path: 'models', element: <AdminModelsPage />, label: 'Models', samplePath: '/admin/models' },
  { area: 'admin', path: 'routes', element: <AdminRoutesPage />, label: 'Model Routes', samplePath: '/admin/routes' },
  { area: 'admin', path: 'plans', element: <AdminPlansPage />, label: 'Plans', samplePath: '/admin/plans' },
  { area: 'admin', path: 'billing', element: <AdminBillingPage />, label: 'Billing', samplePath: '/admin/billing' },
  { area: 'admin', path: 'api-tokens', element: <AdminAPITokensPage />, label: 'API Tokens', samplePath: '/admin/api-tokens' },
  { area: 'admin', path: 'usage-logs', element: <AdminUsageLogsPage />, label: 'Usage Logs', samplePath: '/admin/usage-logs' },
  { area: 'admin', path: 'alerts', element: <AdminAlertsPage />, label: 'Alerts', samplePath: '/admin/alerts' },
  { area: 'admin', path: 'settings', element: <AdminSettingsPage />, label: 'Settings', samplePath: '/admin/settings' },
  { area: 'admin', path: 'users', element: <AdminUsersPage />, label: 'Users', samplePath: '/admin/users' },
  { area: 'admin', path: 'audit-log', element: <AdminAuditLogPage />, label: 'Audit Log', samplePath: '/admin/audit-log' },
  { area: 'admin', path: 'reviews', element: <AdminReviewsPage />, label: 'Review Queue', samplePath: '/admin/reviews' },
];

export const appRouteEntries = [
  ...marketingRouteEntries,
  ...workspaceRouteEntries,
  ...consoleRouteEntries,
  ...adminRouteEntries,
];

function routeObjects(entries: AppRouteEntry[]): RouteObject[] {
  return entries.map(({ area: _area, label: _label, samplePath: _samplePath, ...route }) => route);
}

const routes: RouteObject[] = [
  {
    element: <MarketingLayout />,
    children: routeObjects(marketingRouteEntries)
  },
  {
    element: <ProtectedRoute />,
    children: [
      {
        element: <WorkspaceLayout />,
        children: routeObjects(workspaceRouteEntries)
      },
      {
        path: '/console',
        element: <ConsoleLayout />,
        children: routeObjects(consoleRouteEntries)
      },
      {
        element: <AdminRoute />,
        children: [
          {
            path: '/admin',
            element: <AdminLayout />,
            children: routeObjects(adminRouteEntries)
          }
        ]
      }
    ]
  }
];

export function createAppRouter(initialEntries?: string[]) {
  if (initialEntries && initialEntries.length > 0) {
    return createMemoryRouter(routes, { initialEntries, future: routerFuture });
  }

  return createBrowserRouter(routes, { future: routerFuture });
}
