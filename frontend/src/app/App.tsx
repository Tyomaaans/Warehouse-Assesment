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
import { ProductsPage } from '../features/products/ProductsPage';
import { ReceivingListPage } from '../features/receiving/ReceivingListPage';
import { ReceivingDetailPage } from '../features/receiving/ReceivingDetailPage';
import { CreateReceivingPage } from '../features/receiving/CreateReceivingPage';

export function App() {
  return <QueryClientProvider client={queryClient}><AuthProvider><BrowserRouter><Routes>
    <Route path="/login" element={<LoginPage />} />
    <Route element={<ProtectedRoute />}><Route element={<AppShell />}>
      <Route index element={<OverviewPage />} />
      <Route path="/products" element={<ProductsPage />} />
      <Route path="/receivings" element={<ReceivingListPage />} />
      <Route element={<ProtectedRoute roles={['OPERATOR']} />}><Route path="/receivings/new" element={<CreateReceivingPage />} /></Route>
      <Route path="/receivings/:id" element={<ReceivingDetailPage />} />
      {areas.filter(area => !['/', '/products', '/receivings'].includes(area.path)).map(area => <Route key={area.path} path={area.path} element={<FeaturePlaceholder {...area} planned={area.planned!} />} />)}
      <Route element={<ProtectedRoute roles={['SUPERVISOR']} />}><Route path={reviewArea.path} element={<FeaturePlaceholder {...reviewArea} />} /></Route>
      <Route path="*" element={<EmptyState title="This page isn’t here.">Return to the <Link className="text-link" to="/">workspace overview</Link> to find your way.</EmptyState>} />
    </Route></Route>
  </Routes></BrowserRouter></AuthProvider></QueryClientProvider>;
}
