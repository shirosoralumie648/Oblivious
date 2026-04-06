import {
  createBrowserRouter,
  createMemoryRouter,
  type RouteObject
} from 'react-router-dom';

import { ProtectedRoute } from '../features/auth/ProtectedRoute';
import { ConsoleLayout } from '../features/layouts/ConsoleLayout';
import { MarketingLayout } from '../features/layouts/MarketingLayout';
import { WorkspaceLayout } from '../features/layouts/WorkspaceLayout';
import { AccessPage } from '../routes/console/AccessPage';
import { BillingPage } from '../routes/console/BillingPage';
import { ConsoleHomePage } from '../routes/console/ConsoleHomePage';
import { ModelsPage } from '../routes/console/ModelsPage';
import { UsagePage } from '../routes/console/UsagePage';
import { HomePage } from '../routes/marketing/HomePage';
import { LoginPage } from '../routes/marketing/LoginPage';
import { RegisterPage } from '../routes/marketing/RegisterPage';
import { ChatPage } from '../routes/workspace/ChatPage';
import { KnowledgePage } from '../routes/workspace/KnowledgePage';
import { OnboardingPage } from '../routes/workspace/OnboardingPage';
import { SettingsPage } from '../routes/workspace/SettingsPage';
import { SoloPage } from '../routes/workspace/SoloPage';

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
          { path: '/solo', element: <SoloPage /> },
          { path: '/solo/new', element: <SoloPage /> },
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
          { path: 'access', element: <AccessPage /> }
        ]
      }
    ]
  }
];

export function createAppRouter(initialEntries?: string[]) {
  if (initialEntries && initialEntries.length > 0) {
    return createMemoryRouter(routes, { initialEntries });
  }

  return createBrowserRouter(routes);
}
