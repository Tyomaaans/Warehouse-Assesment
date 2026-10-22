import { api } from '../../lib/api/client';
import type { PaginationMeta } from '../../lib/api/types';

export type TransactionStatus = 'PENDING_APPROVAL' | 'APPROVED' | 'REJECTED_BY_SYSTEM';
export interface ActualItem { receiving_document_item_id: string; actual_quantity: bigint }
export interface TransactionItem extends ActualItem { id: string }
export interface Transaction {
  id: string; receiving_document_id: string; status: TransactionStatus;
  review_status: 'UNRESOLVED' | 'RESOLVED' | null; rejection_reason: string | null; review_note: string | null;
  created_by: string; created_at: string; updated_at: string;
  approved_by: string | null; approved_at: string | null; corrected_by: string | null; corrected_at: string | null;
  reviewed_by: string | null; reviewed_at: string | null; items: TransactionItem[];
}
export interface TransactionsPage { transactions: Transaction[]; meta: PaginationMeta }
const transactionPath = (id: string) => `/transactions/${encodeURIComponent(id)}`;
export const transactionsApi = {
  list: (receiving: string, page: number, limit: number, signal?: AbortSignal) => api.request<TransactionsPage>(`/receivings/${encodeURIComponent(receiving)}/transactions?page=${page}&limit=${limit}`, { signal }),
  detail: (id: string, signal?: AbortSignal) => api.request<Transaction>(transactionPath(id), { signal }),
  create: (receiving: string, items: ActualItem[]) => api.request<Transaction>(`/receivings/${encodeURIComponent(receiving)}/transactions`, { method: 'POST', body: { items } }),
  correct: (id: string, items: ActualItem[], reason: string) => api.request<Transaction>(`${transactionPath(id)}/correct`, { method: 'PATCH', body: { reason, items } }),
  review: (id: string, note: string) => api.request<Transaction>(`${transactionPath(id)}/review`, { method: 'POST', body: { note } }),
  approve: (id: string) => api.request<Transaction>(`${transactionPath(id)}/approve`, { method: 'POST' }),
};
export const isUnresolved = (transaction: Transaction) => transaction.status === 'REJECTED_BY_SYSTEM' && transaction.review_status === 'UNRESOLVED';
export const isActive = (transaction: Transaction) => transaction.status === 'PENDING_APPROVAL' || isUnresolved(transaction);
