import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { AuthContext } from '../auth/context';
import { InventoryPage } from './InventoryPage';
import { inventoryApi } from './api';
import { NotificationsPage } from '../notifications/NotificationsPage';
import { notificationsApi, notificationPolling, type Notification } from '../notifications/api';
import { WarehouseSummary } from '../overview/WarehouseSummary';
import { receivingApi } from '../receiving/api';
import { decodeJSON } from '../../lib/api/json';
import { ApiError, type Role } from '../../lib/api/types';
import { queryKeys } from '../../app/query-client';

const id = 'VQ6EAOKbQdSnFkRmVUQAAA';
const other = 'ZQ6EAOKbQdSnFkRmVUQAAA';
const date = '2026-10-23T00:00:00Z';
const meta = { current_page: 1, page_size: 10, total_pages: 2, total_data: 11 };
const balance = { id, product_id: id, sku: 'SKU-REAL', name: 'Cartons', quantity: 9223372036854775807n, updated_at: date };
const notice = { id, recipient_id: id, source_event_id: other, event_type: 'TRANSACTION_APPROVED', receiving_transaction_id: id, message: 'Your receipt was approved.', read_at: null, created_at: date };
const caches: QueryClient[] = [];
function view(component: React.ReactNode, path = '/inventory', role: Role = 'OPERATOR', actor = id) {
  const cache = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } }); caches.push(cache);
  render(<QueryClientProvider client={cache}><AuthContext.Provider value={{ status: 'authenticated', user: { id: actor, role, expiresAt: Date.now() + 900000 }, signingOut: false, logoutNotice: null, login: vi.fn(), logout: vi.fn(), restore: vi.fn() }}><MemoryRouter initialEntries={[path]}>{component}</MemoryRouter></AuthContext.Provider></QueryClientProvider>);
  return cache;
}
beforeEach(() => {
  vi.spyOn(inventoryApi, 'balances').mockResolvedValue({ balances: [balance], meta });
  vi.spyOn(inventoryApi, 'movements').mockResolvedValue({ movements: [], meta: { ...meta, total_data: 0, total_pages: 0 } });
  vi.spyOn(notificationsApi, 'list').mockResolvedValue({ notifications: [notice], meta });
});
afterEach(() => { cleanup(); caches.splice(0).forEach(cache => cache.clear()); vi.restoreAllMocks(); });

describe('approved inventory', () => {
  it.each(['OPERATOR', 'SUPERVISOR'] as const)('shows exact balances and contract search/pagination for %s', async role => {
    view(<InventoryPage />, '/inventory', role);
    expect(await screen.findByText('9,223,372,036,854,775,807')).toBeTruthy();
    expect(screen.queryByRole('button', { name: /edit|adjust/i })).toBeNull();
    await userEvent.type(screen.getByLabelText('Search inventory'), 'SKU-REAL');
    await userEvent.click(screen.getByRole('button', { name: 'Search' }));
    await waitFor(() => expect(inventoryApi.balances).toHaveBeenLastCalledWith(1, 10, 'SKU-REAL', expect.any(AbortSignal)));
    await userEvent.click(screen.getByRole('button', { name: 'Next' }));
    await waitFor(() => expect(inventoryApi.balances).toHaveBeenLastCalledWith(2, 10, 'SKU-REAL', expect.any(AbortSignal)));
    await userEvent.click(screen.getByRole('link', { name: 'Inbound history' }));
    await userEvent.click(screen.getByRole('link', { name: 'Balances' }));
    expect(screen.getByLabelText('Search inventory')).toHaveProperty('value', '');
  });
  it('joins ledger SKU context to the actual source and links its unchanged identifiers', async () => {
    vi.mocked(inventoryApi.movements).mockResolvedValue({ movements: [{ id, product_id: id, receiving_transaction_item_id: id, receiving_transaction_id: other, receiving_document_id: id, quantity_delta: 9223372036854775807n, created_at: date }], meta });
    vi.spyOn(receivingApi, 'detail').mockResolvedValue({ id, order_reference: 'ORD', status: 'COMPLETED', created_by: id, created_at: date, updated_at: date, completed_at: date, items: [{ id, product_id: id, sku: 'SKU-SOURCE', name: 'Cartons', expected_quantity: 9223372036854775807n, approved_quantity: 9223372036854775807n, remaining_quantity: 0n }] });
    view(<InventoryPage />, `/inventory?view=history&product_id=${id}`);
    expect(await screen.findByText('SKU-SOURCE')).toBeTruthy();
    expect(screen.getByText('+9,223,372,036,854,775,807')).toBeTruthy();
    expect(screen.getByRole('link', { name: other }).getAttribute('href')).toBe(`/transactions/${other}`);
    expect(inventoryApi.movements).toHaveBeenCalledWith(1, 10, id, expect.any(AbortSignal));
  });
  it('rejects invalid product IDs without requesting movements', () => {
    view(<InventoryPage />, '/inventory?view=history&product_id=bad');
    expect(screen.getByText('Invalid product identifier')).toBeTruthy(); expect(inventoryApi.movements).not.toHaveBeenCalled();
  });
  it('shows loading, service failure/retry, and actual empty state', async () => {
    vi.mocked(inventoryApi.balances).mockReturnValueOnce(new Promise(() => {})); view(<InventoryPage />);
    expect(screen.getByRole('status').textContent).toContain('Loading inventory'); cleanup();
    vi.mocked(inventoryApi.balances).mockRejectedValueOnce(new ApiError(503, 'unavailable', 'Inventory unavailable')); view(<InventoryPage />);
    expect(await screen.findByText('Inventory unavailable')).toBeTruthy();
    vi.mocked(inventoryApi.balances).mockResolvedValue({ balances: [], meta: { ...meta, total_data: 0, total_pages: 0 } });
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }));
    expect(await screen.findByText('No inventory on this page')).toBeTruthy();
  });
  it('decodes balances and ledger int64 values without Number rounding', () => {
    expect(decodeJSON('{"quantity":9223372036854775807,"quantity_delta":9223372036854775806}')).toEqual({ quantity: 9223372036854775807n, quantity_delta: 9223372036854775806n });
  });
});
describe('own notifications and dashboard', () => {
  it('marks read once, invalidates only the actor cache and uses bounded visible-view polling', async () => {
    let release!: (value: Notification) => void;
    const read = vi.spyOn(notificationsApi, 'read').mockReturnValue(new Promise(resolve => { release = resolve; }));
    const cache = view(<NotificationsPage />, '/notifications'); const invalidate = vi.spyOn(cache, 'invalidateQueries');
    await userEvent.dblClick(await screen.findByRole('button', { name: 'Mark as read: Your receipt was approved.' }));
    expect(read).toHaveBeenCalledExactlyOnceWith(id);
    vi.mocked(notificationsApi.list).mockResolvedValue({ notifications: [{ ...notice, read_at: date }], meta });
    release({ ...notice, read_at: date });
    expect(await screen.findByText('Read', { exact: true })).toBeTruthy();
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.notifications(id) });
    expect(notificationPolling).toEqual({ refetchInterval: 60000, refetchIntervalInBackground: false });
    expect(cache.getQueryCache().getAll().every(query => query.queryKey[0] === id)).toBe(true);
  });
  it('refreshes stale ownership errors without marking the row read optimistically', async () => {
    vi.spyOn(notificationsApi, 'read').mockRejectedValue(new ApiError(404, 'not_found', 'Notification not found'));
    view(<NotificationsPage />, '/notifications', 'SUPERVISOR', other);
    await userEvent.click(await screen.findByRole('button', { name: 'Mark as read: Your receipt was approved.' }));
    expect(await screen.findByText('Notification not found')).toBeTruthy(); expect(screen.getByText('Unread', { exact: true })).toBeTruthy();
  });
  it('uses metadata totals rather than preview length and isolates partial dashboard errors', async () => {
    vi.spyOn(receivingApi, 'list').mockImplementation(async filters => ({ receivings: [], meta: { ...meta, total_data: filters.status === 'COMPLETED' ? 31 : 23 } }));
    vi.mocked(inventoryApi.balances).mockRejectedValue(new ApiError(503, 'unavailable', 'Stock offline'));
    view(<WarehouseSummary />);
    expect(await screen.findByText('31')).toBeTruthy(); expect(screen.getByText('23')).toBeTruthy();
    expect(await screen.findByText('Stock offline')).toBeTruthy(); expect(screen.getByText('Your receipt was approved.')).toBeTruthy();
    expect(screen.getByText('11')).toBeTruthy(); expect(screen.queryByText(/total unread/i)).toBeNull();
  });
});
