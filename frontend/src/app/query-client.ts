import { QueryClient } from '@tanstack/react-query';
import { ApiError } from '../lib/api/types';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 30_000, retry: (count, error) => count < 1 && !(error instanceof ApiError && error.status >= 400 && error.status < 500), refetchOnWindowFocus: false },
    mutations: { retry: false },
  },
});

// Feature keys include the actor so later business caches cannot cross user sessions.
export const queryKeys = {
  products: (actor: string) => [actor, 'products'] as const,
  receivings: (actor: string) => [actor, 'receivings'] as const,
  transactions: (actor: string, receiving: string) => [actor, 'transactions', receiving] as const,
  inventory: (actor: string) => [actor, 'inventory'] as const,
  notifications: (actor: string) => [actor, 'notifications'] as const,
};
