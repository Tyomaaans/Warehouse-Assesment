import { BrowserRouter, Routes, Route, Link } from 'react-router-dom';
import { QueryClientProvider } from '@tanstack/react-query';
import { AuthProvider } from '../features/auth/AuthProvider';
import { LoginPage } from '../features/auth/LoginPage';
import { ProtectedRoute } from '../features/auth/ProtectedRoute';
import { OverviewPage } from '../features/overview/OverviewPage';
import { AppShell } from '../components/layout/AppShell';
import { EmptyState } from '../components/ui/States';
import { FeaturePlaceholder } from './FeaturePlaceholder';
import { areas, reviewArea } from './navigation';
import { queryClient } from './query-client';

export function App() {
  return <QueryClientProvider client={queryClient}><AuthProvider><BrowserRouter><Routes>
    <Route path="/login" element={<LoginPage />} />
    <Route element={<ProtectedRoute />}><Route element={<AppShell />}>
      <Route index element={<OverviewPage />} />
      {areas.filter(area => area.path !== '/').map(area => <Route key={area.path} path={area.path} element={<FeaturePlaceholder {...area} planned={area.planned!} />} />)}
      <Route element={<ProtectedRoute roles={['SUPERVISOR']} />}><Route path={reviewArea.path} element={<FeaturePlaceholder {...reviewArea} />} /></Route>
      <Route path="*" element={<EmptyState title="This page isn’t here.">Return to the <Link className="text-link" to="/">workspace overview</Link> to find your way.</EmptyState>} />
    </Route></Route>
  </Routes></BrowserRouter></AuthProvider></QueryClientProvider>;
}
