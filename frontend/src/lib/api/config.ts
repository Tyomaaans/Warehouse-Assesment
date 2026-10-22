export function resolveApiBase(value: string | undefined): string {
  const base = (value ?? '/api/v1').trim().replace(/\/+$/, '');
  // The backend has host-only SameSite=Strict cookies and no cross-origin CORS contract.
  if (!base.startsWith('/') || base.startsWith('//') || /[?#\\\s]/.test(base)) {
    throw new Error('VITE_API_BASE_URL must be a same-origin API path. Configure API_PROXY_TARGET for development.');
  }
  return base;
}

export const API_BASE = resolveApiBase(import.meta.env.VITE_API_BASE_URL);
