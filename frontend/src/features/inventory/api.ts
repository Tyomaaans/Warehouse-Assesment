import { api } from '../../lib/api/client';
import type { PaginationMeta } from '../../lib/api/types';

export interface Balance { id: string; product_id: string; sku: string; name: string; quantity: bigint; updated_at: string }
export interface Movement { id: string; product_id: string; receiving_transaction_item_id: string; receiving_transaction_id: string; receiving_document_id: string; quantity_delta: bigint; created_at: string }
export interface BalancesPage { balances: Balance[]; meta: PaginationMeta }
export interface MovementsPage { movements: Movement[]; meta: PaginationMeta }
export const inventoryApi = {
  balances: (page: number, limit: number, search = '', signal?: AbortSignal) => api.request<BalancesPage>(`/inventory?${new URLSearchParams({ page: String(page), limit: String(limit), ...(search ? { search } : {}) })}`, { signal }),
  movements: (page: number, limit: number, product = '', signal?: AbortSignal) => api.request<MovementsPage>(`/inventory/transactions?${new URLSearchParams({ page: String(page), limit: String(limit), ...(product ? { product_id: product } : {}) })}`, { signal }),
};
