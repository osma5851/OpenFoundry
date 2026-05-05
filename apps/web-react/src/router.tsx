import { createBrowserRouter } from 'react-router-dom';

import { AppShell } from '@components/AppShell';
import { Home } from './routes/Home';
import { NotFound } from './routes/NotFound';

export const router = createBrowserRouter([
  {
    path: '/',
    element: <AppShell />,
    errorElement: <NotFound />,
    children: [
      { index: true, element: <Home /> },
      // Migration pattern: add a route here as you port each SvelteKit folder under apps/web/src/routes/.
      // Example:
      // { path: 'datasets', lazy: async () => ({ Component: (await import('./routes/datasets/DatasetsPage')).DatasetsPage }) },
      { path: '*', element: <NotFound /> },
    ],
  },
]);
