import { createContext } from 'react';
import type { LoginRequest } from './api';
import type { SessionUser } from './session';

export interface AuthState {
  status: 'restoring' | 'authenticated' | 'anonymous' | 'unavailable';
  user: SessionUser | null;
  message?: string;
}
export interface AuthContextValue extends AuthState {
  signingOut: boolean;
  logoutNotice: string | null;
  login: (request: LoginRequest) => Promise<void>;
  logout: () => Promise<void>;
  restore: () => void;
}
export const AuthContext = createContext<AuthContextValue | null>(null);
