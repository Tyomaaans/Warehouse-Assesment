import { api } from '../../lib/api/client';
import type { PaginationMeta } from '../../lib/api/types';

export interface Notification { id: string; recipient_id: string; source_event_id: string; event_type: string; receiving_transaction_id: string | null; message: string; read_at: string | null; created_at: string }
export interface NotificationsPage { notifications: Notification[]; meta: PaginationMeta }
export const notificationsApi = {
  list: (page: number, limit: number, signal?: AbortSignal) => api.request<NotificationsPage>(`/notifications?page=${page}&limit=${limit}`, { signal }),
  read: (id: string) => api.request<Notification>(`/notifications/${encodeURIComponent(id)}/read`, { method: 'PATCH' }),
};
// Only mounted, visible views poll. No background-tab traffic or global unread total is invented.
export const notificationPolling = { refetchInterval: 60_000, refetchIntervalInBackground: false } as const;
