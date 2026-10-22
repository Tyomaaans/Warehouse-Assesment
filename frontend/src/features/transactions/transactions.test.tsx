import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { AuthContext } from '../auth/context';
import { receivingApi, type ReceivingDetail } from '../receiving/api';
import { ApiClient } from '../../lib/api/client';
import { ApiError, type Role } from '../../lib/api/types';
import { encodeJSON } from '../../lib/api/json';
import { queryKeys } from '../../app/query-client';
import { transactionsApi, type Transaction } from './api';
import { TransactionEntryPage } from './TransactionEntryPage';
import { TransactionDetailPage } from './TransactionDetailPage';
import { TransactionHistory } from './TransactionHistory';

const id = 'VQ6EAOKbQdSnFkRmVUQAAA';
const lineID = 'ZQ6EAOKbQdSnFkRmVUQAAA';
const otherID = 'dQ6EAOKbQdSnFkRmVUQAAA';
const date = '2026-10-23T00:00:00Z';
const receiving: ReceivingDetail = { id, order_reference: 'ORD-RECEIPT', status: 'INCOMPLETE', created_by: id, created_at: date, updated_at: date, completed_at: null, items: [{ id: lineID, product_id: id, sku: 'SKU-A', name: 'Carton', expected_quantity: 10n, approved_quantity: 4n, remaining_quantity: 6n }, { id: otherID, product_id: otherID, sku: 'SKU-B', name: 'Pallet', expected_quantity: 5n, approved_quantity: 0n, remaining_quantity: 5n }] };
const pending: Transaction = { id, receiving_document_id: id, status: 'PENDING_APPROVAL', review_status: null, rejection_reason: null, review_note: null, created_by: id, created_at: date, updated_at: date, approved_by: null, approved_at: null, corrected_by: null, corrected_at: null, reviewed_by: null, reviewed_at: null, items: [{ id: lineID, receiving_document_item_id: lineID, actual_quantity: 6n }] };
const rejected: Transaction = { ...pending, status: 'REJECTED_BY_SYSTEM', review_status: 'UNRESOLVED', rejection_reason: 'actual_quantity exceeds remaining expected quantity', items: [{ ...pending.items[0]!, actual_quantity: 60n }] };
const approved: Transaction = { ...pending, status: 'APPROVED', approved_by: otherID, approved_at: date };
const emptyMeta = { current_page: 1, page_size: 10, total_data: 0, total_pages: 0 };
const clients: QueryClient[] = [];
// jsdom omits native dialog methods; real dialog behavior is verified in Chrome.
beforeAll(() => {
  Object.defineProperty(HTMLDialogElement.prototype, 'showModal', { configurable: true, value: function (this: HTMLDialogElement) { this.setAttribute('open', ''); } });
  Object.defineProperty(HTMLDialogElement.prototype, 'close', { configurable: true, value: function (this: HTMLDialogElement) { this.removeAttribute('open'); } });
});
afterAll(() => { Reflect.deleteProperty(HTMLDialogElement.prototype, 'showModal'); Reflect.deleteProperty(HTMLDialogElement.prototype, 'close'); });
beforeEach(() => {
  vi.spyOn(receivingApi, 'detail').mockResolvedValue(receiving);
  vi.spyOn(transactionsApi, 'detail').mockResolvedValue(pending);
  vi.spyOn(transactionsApi, 'list').mockResolvedValue({ transactions: [], meta: emptyMeta });
});
afterEach(() => { cleanup(); clients.splice(0).forEach(cache => cache.clear()); vi.restoreAllMocks(); });
function view(role: Role, path = `/transactions/${id}`, history = false) {
  const cache = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } }); clients.push(cache);
  render(<QueryClientProvider client={cache}><AuthContext.Provider value={{ status: 'authenticated', user: { id, role, expiresAt: Date.now() + 900_000 }, signingOut: false, logoutNotice: null, login: vi.fn(), logout: vi.fn(), restore: vi.fn() }}><MemoryRouter initialEntries={[path]}>{history ? <TransactionHistory receiving={receiving} /> : <Routes><Route path="/receivings/:id/transactions/new" element={<TransactionEntryPage />} /><Route path="/transactions/:id" element={<TransactionDetailPage />} /></Routes>}</MemoryRouter></AuthContext.Provider></QueryClientProvider>);
  return cache;
}
async function fillActual(value: string, corrected = false) {
  if (!corrected) { const checkbox = await screen.findByLabelText('Include SKU-A'); if (!(checkbox as HTMLInputElement).checked) await userEvent.click(checkbox); }
  const input = screen.getByLabelText(`${corrected ? 'Corrected' : 'Actual'} quantity for SKU-A`);
  await userEvent.clear(input); await userEvent.type(input, value);
}

describe('actual receiving', () => {
  it('submits an Operator partial receipt with RCV item IDs and shows pending confirmation', async () => {
    const create = vi.spyOn(transactionsApi, 'create').mockResolvedValue(pending);
    view('OPERATOR', `/receivings/${id}/transactions/new`);
    await fillActual('6');
    await userEvent.click(screen.getByRole('button', { name: 'Record physical receipt' }));
    expect(await screen.findByText('Pending approval')).toBeTruthy();
    expect(screen.getByRole('status').textContent).toContain('Awaiting Supervisor approval');
    expect(create).toHaveBeenCalledExactlyOnceWith(id, [{ receiving_document_item_id: lineID, actual_quantity: 6n }]);
    expect(screen.queryByRole('button', { name: 'Approve transaction' })).toBeNull();
  });

  it('records full-int64 physical excess and treats the persisted rejected result as success', async () => {
    const excess = { ...rejected, items: [{ ...rejected.items[0]!, actual_quantity: 9223372036854775807n }] };
    const create = vi.spyOn(transactionsApi, 'create').mockResolvedValue(excess);
    vi.mocked(transactionsApi.detail).mockResolvedValue(excess);
    view('OPERATOR', `/receivings/${id}/transactions/new`);
    await fillActual('9223372036854775807');
    expect(screen.getByText(/This exceeds the remaining quantity/)).toBeTruthy();
    await userEvent.click(screen.getByRole('button', { name: 'Record physical receipt' }));
    expect(await screen.findByText('Rejected by system')).toBeTruthy();
    expect(screen.getByText('Unresolved')).toBeTruthy();
    expect(screen.getByRole('status').textContent).toContain('attempt is saved');
    expect(create).toHaveBeenCalledExactlyOnceWith(id, [{ receiving_document_item_id: lineID, actual_quantity: 9223372036854775807n }]);
  });

  it('validates empty/zero/fractional/overflow input and prevents duplicate requests while saving', async () => {
    let release!: (value: Transaction) => void;
    const create = vi.spyOn(transactionsApi, 'create').mockReturnValue(new Promise(resolve => { release = resolve; }));
    view('OPERATOR', `/receivings/${id}/transactions/new`);
    await screen.findByLabelText('Include SKU-A');
    await userEvent.click(screen.getByRole('button', { name: 'Record physical receipt' }));
    expect(screen.getByText('Select at least one SKU received.')).toBeTruthy();
    await fillActual('0');
    for (const value of ['0', '1.5', '9223372036854775808']) {
      const input = screen.getByLabelText('Actual quantity for SKU-A'); await userEvent.clear(input); await userEvent.type(input, value);
      await userEvent.click(screen.getByRole('button', { name: 'Record physical receipt' }));
      expect(screen.getByText('Enter a whole number from 1 to 9223372036854775807.')).toBeTruthy();
    }
    expect(create).not.toHaveBeenCalled();
    await fillActual('6');
    await userEvent.dblClick(screen.getByRole('button', { name: 'Record physical receipt' }));
    expect(create).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('button', { name: 'Saving…' })).toHaveProperty('disabled', true);
    release(pending);
    expect(await screen.findByText('Pending approval')).toBeTruthy();
  });

  it('refreshes active state after a concurrent submission conflict', async () => {
    vi.spyOn(transactionsApi, 'create').mockRejectedValue(new ApiError(409, 'conflict', 'receiving document already has an active transaction'));
    view('OPERATOR', `/receivings/${id}/transactions/new`);
    await fillActual('6');
    vi.mocked(transactionsApi.list).mockResolvedValue({ transactions: [pending], meta: { ...emptyMeta, total_data: 1, total_pages: 1 } });
    await userEvent.click(screen.getByRole('button', { name: 'Record physical receipt' }));
    expect(await screen.findByText('An active transaction already exists')).toBeTruthy();
    expect(screen.getByRole('link', { name: 'Open active transaction' })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Record physical receipt' })).toBeNull();
  });

  it('keeps completed documents closed to transaction entry', async () => {
    vi.mocked(receivingApi.detail).mockResolvedValue({ ...receiving, status: 'COMPLETED', completed_at: date });
    view('OPERATOR', `/receivings/${id}/transactions/new`);
    expect(await screen.findByText('Receiving is completed')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Record physical receipt' })).toBeNull();
  });
});

describe('Supervisor decisions', () => {
  it('shows recorded values beside corrected values and requires an audit reason on the same ID', async () => {
    vi.mocked(transactionsApi.detail).mockResolvedValue(rejected);
    const correct = vi.spyOn(transactionsApi, 'correct').mockResolvedValue(pending);
    view('SUPERVISOR');
    await userEvent.click(await screen.findByRole('button', { name: 'Correct recorded input' }));
    expect(screen.getAllByText('Recorded before correction')).toHaveLength(2);
    await fillActual('6', true);
    await userEvent.click(screen.getByRole('button', { name: 'Save correction' }));
    expect(screen.getByText('Enter the reason for this correction.')).toBeTruthy(); expect(correct).not.toHaveBeenCalled();
    await userEvent.type(screen.getByLabelText('Correction reason'), 'Recorded an extra zero');
    await userEvent.click(screen.getByRole('button', { name: 'Save correction' }));
    expect(await screen.findByText('Pending approval')).toBeTruthy();
    expect(correct).toHaveBeenCalledExactlyOnceWith(id, [{ receiving_document_item_id: lineID, actual_quantity: 6n }], 'Recorded an extra zero');
  });

  it('resolves confirmed excess with a required note and never changes rejected to approved', async () => {
    vi.mocked(transactionsApi.detail).mockResolvedValue(rejected);
    const review = vi.spyOn(transactionsApi, 'review').mockResolvedValue({ ...rejected, review_status: 'RESOLVED', review_note: 'Physical excess confirmed', reviewed_by: otherID, reviewed_at: date });
    view('SUPERVISOR');
    await userEvent.click(await screen.findByRole('button', { name: 'Resolve confirmed excess' }));
    await userEvent.click(screen.getByRole('button', { name: 'Resolve review' }));
    expect(screen.getByText('Enter a verification note for the confirmed physical excess.')).toBeTruthy();
    await userEvent.type(screen.getByLabelText('Verification note'), 'Physical excess confirmed');
    await userEvent.click(screen.getByRole('button', { name: 'Resolve review' }));
    expect(await screen.findByText('Resolved', { exact: true })).toBeTruthy();
    expect(screen.getByText('Rejected by system')).toBeTruthy();
    expect(review).toHaveBeenCalledExactlyOnceWith(id, 'Physical excess confirmed');
    expect(screen.queryByRole('button', { name: 'Correct recorded input' })).toBeNull();
  });

  it('requires approval confirmation, locks repeated clicks and invalidates inventory/receiving after success', async () => {
    let release!: (value: Transaction) => void;
    const approve = vi.spyOn(transactionsApi, 'approve').mockReturnValue(new Promise(resolve => { release = resolve; }));
    const cache = view('SUPERVISOR');
    cache.setQueryData(queryKeys.inventory(id), { quantity: 4n });
    const invalidate = vi.spyOn(cache, 'invalidateQueries');
    await userEvent.click(await screen.findByRole('button', { name: 'Approve transaction' }));
    expect(screen.getByRole('dialog', { name: 'Approve receiving transaction?' })).toBeTruthy(); expect(approve).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' })); expect(approve).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole('button', { name: 'Approve transaction' }));
    await userEvent.dblClick(screen.getByRole('button', { name: 'Confirm approval' }));
    expect(approve).toHaveBeenCalledExactlyOnceWith(id);
    expect(cache.getQueryData(queryKeys.inventory(id))).toEqual({ quantity: 4n });
    release(approved);
    expect(await screen.findByText('Approved', { exact: true })).toBeTruthy();
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.inventory(id) });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.receivings(id) });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.transactions(id, id) });
    expect(screen.getByText('Incomplete', { exact: true })).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Approve transaction' })).toBeNull();
  });

  it('refreshes stale transaction state on 409, displays the conflict and keeps approved data immutable', async () => {
    vi.spyOn(transactionsApi, 'approve').mockRejectedValue(new ApiError(409, 'conflict', 'only pending transactions can be approved'));
    view('SUPERVISOR');
    await userEvent.click(await screen.findByRole('button', { name: 'Approve transaction' }));
    vi.mocked(transactionsApi.detail).mockResolvedValue(approved);
    await userEvent.click(screen.getByRole('button', { name: 'Confirm approval' }));
    expect(await screen.findByText('Approved transactions are immutable.')).toBeTruthy();
    expect(screen.getByRole('alert').textContent).toContain('only pending transactions');
    expect(screen.queryByRole('button', { name: 'Approve transaction' })).toBeNull();
  });

  it('hides all Supervisor decisions from Operator and all corrections from approved transactions', async () => {
    vi.mocked(transactionsApi.detail).mockResolvedValue(rejected); view('OPERATOR');
    await screen.findByText('Rejected by system');
    expect(screen.queryByRole('button', { name: 'Correct recorded input' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Resolve confirmed excess' })).toBeNull();
    cleanup(); vi.mocked(transactionsApi.detail).mockResolvedValue(approved); view('SUPERVISOR');
    await screen.findByText('Approved transactions are immutable.');
    expect(screen.queryByRole('button', { name: 'Correct recorded input' })).toBeNull();
  });
});

describe('history, errors and numeric contract', () => {
  it('loads paginated history, shows review states and prevents entry when an active receipt is visible', async () => {
    vi.mocked(transactionsApi.list).mockResolvedValue({ transactions: [rejected], meta: { ...emptyMeta, total_pages: 2, total_data: 11 } });
    view('OPERATOR', '/', true);
    expect(await screen.findByText('Rejected by system')).toBeTruthy();
    expect(screen.getByText('Unresolved')).toBeTruthy();
    expect(screen.queryByRole('link', { name: 'Record physical receipt' })).toBeNull();
    await userEvent.click(screen.getByRole('button', { name: 'Next' }));
    await waitFor(() => expect(transactionsApi.list).toHaveBeenLastCalledWith(id, 2, 10, expect.any(AbortSignal)));
  });

  it('renders loading, error and invalid-identifier states without fabricating transaction data', async () => {
    vi.mocked(transactionsApi.detail).mockReturnValue(new Promise(() => {})); view('OPERATOR');
    expect(screen.getByRole('status').textContent).toContain('Loading transaction'); cleanup();
    vi.mocked(transactionsApi.detail).mockRejectedValue(new ApiError(503, 'unavailable', 'Service temporarily unavailable')); view('OPERATOR');
    expect(await screen.findByText('Service temporarily unavailable')).toBeTruthy(); cleanup();
    const calls = vi.mocked(transactionsApi.detail).mock.calls.length; view('OPERATOR', '/transactions/bad-id');
    expect(screen.getByText('Invalid transaction identifier')).toBeTruthy(); expect(transactionsApi.detail).toHaveBeenCalledTimes(calls);
  });

  it('preserves actual_quantity at int64 maximum in persisted HTTP 201 rejected responses', async () => {
    const excess = { ...rejected, items: [{ ...rejected.items[0]!, actual_quantity: 9223372036854775807n }] };
    const client = new ApiClient('/api/v1', vi.fn<typeof fetch>(async (_url, options) => {
      expect(options?.body).toContain('"actual_quantity":9223372036854775807');
      return new Response(encodeJSON({ success: true, data: excess }), { status: 201 });
    }));
    const result = await client.request<Transaction>(`/receivings/${id}/transactions`, { method: 'POST', body: { items: [{ receiving_document_item_id: lineID, actual_quantity: 9223372036854775807n }] } });
    expect(result.status).toBe('REJECTED_BY_SYSTEM'); expect(result.review_status).toBe('UNRESOLVED');
    expect(result.items[0]!.actual_quantity).toBe(9223372036854775807n);
  });
});
