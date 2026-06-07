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
import { HomePage } from '../routes/marketing/HomePage';
import { LoginPage } from '../routes/marketing/LoginPage';
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

const routes: RouteObject[] = [
  {
    element: <MarketingLayout />,
    children: [
      { path: '/', element: <HomePage /> },
      { path: '/login', element: <LoginPage /> },
      { path: '/register', element: <RegisterPage /> }
    ]
  },
  {
    element: <ProtectedRoute />,
    children: [
      {
        element: <WorkspaceLayout />,
        children: [
          { path: '/onboarding', element: <OnboardingPage /> },
          { path: '/chat', element: <ChatPage /> },
          { path: '/chat/:conversationId', element: <ChatPage /> },
          { path: '/knowledge', element: <KnowledgePage /> },
          { path: '/knowledge/:knowledgeBaseId', element: <KnowledgePage /> },
          { path: '/agents', element: <AgentsPage /> },
          { path: '/mcp-servers', element: <McpServersPage /> },
          { path: '/memories', element: <AgentMemoriesPage /> },
          { path: '/agent-runs/:runId/plan-steps', element: <AgentPlanStepsPage /> },
          { path: '/solo', element: <SoloPage /> },
          { path: '/solo/new', element: <SoloPage /> },
          { path: '/workflows', element: <WorkflowsPage /> },
          { path: '/scheduled-tasks', element: <ScheduledTasksPage /> },
          { path: '/publishing', element: <PublishingChannelsPage /> },
          { path: '/marketplace', element: <MarketplaceHomePage /> },
          { path: '/marketplace/agents/:agentId', element: <MarketplaceAgentDetailPage /> },
          { path: '/marketplace/publish', element: <MarketplacePublishPage /> },
          { path: '/marketplace/my-agents', element: <MarketplaceMyAgentsPage /> },
          { path: '/settings', element: <SettingsPage /> }
        ]
      },
      {
        path: '/console',
        element: <ConsoleLayout />,
        children: [
          { index: true, element: <ConsoleHomePage /> },
          { path: 'models', element: <ModelsPage /> },
          { path: 'usage', element: <UsagePage /> },
          { path: 'billing', element: <BillingPage /> },
          { path: 'access', element: <AccessPage /> },
          { path: 'notifications', element: <NotificationsPage /> }
        ]
      },
      {
        element: <AdminRoute />,
        children: [
          {
            path: '/admin',
            element: <AdminLayout />,
            children: [
              { index: true, element: <AdminHomePage /> },
              { path: 'channels', element: <AdminChannelsPage /> },
              { path: 'models', element: <AdminModelsPage /> },
              { path: 'routes', element: <AdminRoutesPage /> },
              { path: 'plans', element: <AdminPlansPage /> },
              { path: 'billing', element: <AdminBillingPage /> },
              { path: 'api-tokens', element: <AdminAPITokensPage /> },
              { path: 'usage-logs', element: <AdminUsageLogsPage /> },
              { path: 'alerts', element: <AdminAlertsPage /> },
              { path: 'settings', element: <AdminSettingsPage /> },
              { path: 'users', element: <AdminUsersPage /> },
              { path: 'audit-log', element: <AdminAuditLogPage /> },
              { path: 'reviews', element: <AdminReviewsPage /> }
            ]
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
