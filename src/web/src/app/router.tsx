import {
  createBrowserRouter,
  createMemoryRouter,
  type RouteObject
} from 'react-router-dom';

function HomePage() {
  return (
    <main>
      <h1>Oblivious</h1>
      <p>AI workspace framework</p>
    </main>
  );
}

const routes: RouteObject[] = [
  {
    path: '/',
    element: <HomePage />
  }
];

export function createAppRouter(initialEntries?: string[]) {
  if (initialEntries) {
    return createMemoryRouter(routes, { initialEntries });
  }

  return createBrowserRouter(routes);
}
