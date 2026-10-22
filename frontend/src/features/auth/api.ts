import { api } from '../../lib/api/client';
import type { LoginData } from '../../lib/api/types';

export interface LoginRequest { email: string; password: string; remember_me: boolean }
export const authApi = {
  login: (body: LoginRequest) => api.request<LoginData>('/auth/login', { method: 'POST', body, public: true }),
  restore: () => api.refresh(),
  logout: () => api.request<void>('/auth/logout', { method: 'POST' }),
};
