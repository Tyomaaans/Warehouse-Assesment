import { api } from '../../lib/api/client';
import type { PaginationMeta } from '../../lib/api/types';

export type ReceivingStatus = 'INCOMPLETE' | 'COMPLETED';
export interface ReceivingSummary {
  id: string; order_reference: string; status: ReceivingStatus;
  created_by: string; created_at: string; updated_at: string; completed_at: string | null;
}
export interface ReceivingItem {
  id: string; product_id: string; sku: string; name: string;
  expected_quantity: bigint; approved_quantity: bigint; remaining_quantity: bigint;
}
export interface ReceivingDetail extends ReceivingSummary { items: ReceivingItem[] }
export interface ReceivingsPage { receivings: ReceivingSummary[]; meta: PaginationMeta }
export interface ReceivingInput { order_reference: string; items: { product_id: string; expected_quantity: bigint }[] }
export interface ReceivingFilters { page: number; limit: number; status: ReceivingStatus | '' }

export const receivingApi = {
  list: (filters: ReceivingFilters, signal?: AbortSignal) => {
    const query = new URLSearchParams({ page: String(filters.page), limit: String(filters.limit) });
    if (filters.status) query.set('status', filters.status);
    return api.request<ReceivingsPage>(`/receivings?${query}`, { signal });
  },
  detail: (id: string, signal?: AbortSignal) => api.request<ReceivingDetail>(`/receivings/${encodeURIComponent(id)}`, { signal }),
  create: (body: ReceivingInput) => api.request<ReceivingDetail>('/receivings', { method: 'POST', body }),
};

export function isCanonicalID(value: string): boolean {
  if (!/^[A-Za-z0-9_-]{21}[AQgw]$/.test(value)) return false;
  const bytes = atob(value.replace(/-/g, '+').replace(/_/g, '/'));
  return bytes.length === 16 && (bytes.charCodeAt(6) >> 4) === 4 && (bytes.charCodeAt(8) & 0xc0) === 0x80 &&
    btoa(bytes).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '') === value;
}

export function positiveQuantity(value: string): bigint | null {
  if (!/^\d+$/.test(value) || value.length > 19) return null;
  const quantity = BigInt(value);
  return quantity >= 1n && quantity <= 9223372036854775807n ? quantity : null;
}
