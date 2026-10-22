import { Navigate, Outlet, useLocation, Link } from 'react-router-dom';
import type { Role } from '../../lib/api/types';
import { Button } from '../../components/ui/Button';
import { ErrorState, LoadingState } from '../../components/ui/States';
import { useAuth } from './use-auth';

export function ProtectedRoute({ roles }: { roles?: Role[] }) {
  const auth = useAuth();
  const location = useLocation();
  if (auth.status === 'restoring' || auth.signingOut) return <LoadingState label={auth.signingOut ? 'Signing out…' : 'Restoring your session…'} />;
  if (auth.status === 'unavailable') return <div className="session-gate"><ErrorState title="We couldn’t restore your session" action={<Button onClick={auth.restore}>Try again</Button>}>{auth.message ?? 'The server is temporarily unavailable. Your workspace will open once your session can be verified.'}</ErrorState></div>;
  if (!auth.user) return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  if (roles && !roles.includes(auth.user.role)) return <ErrorState title={`${roles.includes('SUPERVISOR') ? 'Supervisor' : 'Operator'} access required`} action={<Link className="button button-secondary" to="/">Back to overview</Link>}>Your role does not have access to this workspace.</ErrorState>;
  return <Outlet />;
}
