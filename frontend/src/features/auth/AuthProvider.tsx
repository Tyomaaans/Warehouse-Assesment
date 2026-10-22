import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api } from '../../lib/api/client';
import { ApiError } from '../../lib/api/types';
import { authApi, type LoginRequest } from './api';
import { AuthContext, type AuthState } from './context';
import { userFromToken } from './session';

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [state, setState] = useState<AuthState>({ status: 'restoring', user: null });
  const [signingOut, setSigningOut] = useState(false);
  const [logoutNotice, setLogoutNotice] = useState<string | null>(null);

  useEffect(() => {
    const unsubscribe = api.subscribe((token, error) => {
      if (!token) {
        queryClient.clear();
        const unavailable = error && ![400, 401, 403].includes(error.status);
        setState({ status: unavailable ? 'unavailable' : 'anonymous', user: null, message: error?.message });
        return;
      }
      try {
        const user = userFromToken(token);
        setState(previous => ({ status: 'authenticated', user: { ...user, email: previous.user?.id === user.id ? previous.user.email : undefined } }));
      } catch (error) { api.clearSession(error as ApiError); }
    });
    void authApi.restore().catch(() => { /* The session listener supplies the visible state. */ });
    return unsubscribe;
  }, [queryClient]);

  const expiresAt = state.user?.expiresAt;
  useEffect(() => {
    if (!expiresAt) return;
    const timer = setTimeout(() => { void authApi.restore().catch(() => {}); }, Math.max(expiresAt - Date.now() - 30_000, 5_000));
    return () => clearTimeout(timer);
  }, [expiresAt]);

  const login = useCallback(async (request: LoginRequest) => {
    const result = await authApi.login(request);
    const user = userFromToken(result.access_token);
    queryClient.clear();
    setLogoutNotice(null);
    api.setAccessToken(result.access_token);
    setState({ status: 'authenticated', user: { ...user, email: result.user.email } });
  }, [queryClient]);

  const logout = useCallback(async () => {
    setSigningOut(true);
    setLogoutNotice(null);
    try { await authApi.logout(); }
    catch (error) {
      if (!(error instanceof ApiError) || error.status !== 401) {
        setLogoutNotice('You are signed out on this page, but server sign-out could not be confirmed. Retry sign out when the connection is available.');
      }
    } finally {
      api.clearSession();
      queryClient.clear();
      setSigningOut(false);
    }
  }, [queryClient]);

  const restore = useCallback(() => {
    setState({ status: 'restoring', user: null });
    void authApi.restore().catch(() => {});
  }, []);

  return <AuthContext.Provider value={{ ...state, signingOut, logoutNotice, login, logout, restore }}>{children}</AuthContext.Provider>;
}
