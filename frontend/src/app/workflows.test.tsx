import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import type { ReactNode } from 'react';
import { AuthContext } from '../features/auth/context';
import { ProtectedRoute } from '../features/auth/ProtectedRoute';
import { ProductsPage } from '../features/products/ProductsPage';
import { productsApi, type Product } from '../features/products/api';
import { ReceivingListPage } from '../features/receiving/ReceivingListPage';
import { ReceivingDetailPage } from '../features/receiving/ReceivingDetailPage';
import { CreateReceivingPage } from '../features/receiving/CreateReceivingPage';
import { receivingApi, positiveQuantity, isCanonicalID, type ReceivingDetail } from '../features/receiving/api';
import { ApiError, type Role } from '../lib/api/types';
import { ApiClient } from '../lib/api/client';
import { decodeJSON, encodeJSON } from '../lib/api/json';
import { api } from '../lib/api/client';

const firstID = 'VQ6EAOKbQdSnFkRmVUQAAA';
const secondID = 'ZQ6EAOKbQdSnFkRmVUQAAA';
const date = '2026-10-23T00:00:00Z';
const first: Product = { id: firstID, sku: 'SKU-A', name: 'Carton', created_at: date };
const second: Product = { ...first, id: secondID, sku: 'SKU-B', name: 'Pallet' };
const meta = { current_page: 1, total_pages: 1, page_size: 10, total_data: 1 };
const document: ReceivingDetail = { id: firstID, order_reference: 'ORD-24', status: 'INCOMPLETE', created_by: firstID, created_at: date, updated_at: date, completed_at: null, items: [{ id: secondID, product_id: firstID, sku: 'SKU-A', name: 'Carton', expected_quantity: 7n, approved_quantity: 5n, remaining_quantity: 2n }] };
const clients: QueryClient[] = [];
afterEach(() => { cleanup(); clients.splice(0).forEach(client => client.clear()); vi.restoreAllMocks(); api.clearSession(); });

function view(node: ReactNode, role: Role = 'OPERATOR', path = '/') {
  const cache = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  clients.push(cache);
  render(<QueryClientProvider client={cache}><AuthContext.Provider value={{ status: 'authenticated', user: { id: firstID, role, expiresAt: Date.now() + 900_000 }, signingOut: false, logoutNotice: null, login: vi.fn(), logout: vi.fn(), restore: vi.fn() }}><MemoryRouter initialEntries={[path]}>{node}</MemoryRouter></AuthContext.Provider></QueryClientProvider>);
  return cache;
}
function catalogue() { return vi.spyOn(productsApi, 'list').mockResolvedValue({ products: [first, second], meta: { ...meta, total_data: 2 } }); }
async function line(index: number, product: string, quantity: string) {
  await userEvent.selectOptions(await screen.findByLabelText(`Product for line ${index}`), product);
  await userEvent.type(screen.getByLabelText(`Expected quantity for line ${index}`), quantity);
}

describe('product management', () => {
  it('lists real query data for Operator, with search/pagination and no create action', async () => {
    const list = catalogue();
    view(<ProductsPage />);
    expect(await screen.findByText('SKU-A')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'New product' })).toBeNull();
    await userEvent.type(screen.getByLabelText('Search products'), 'SKU_%');
    await userEvent.click(screen.getByRole('button', { name: 'Search' }));
    await waitFor(() => expect(list).toHaveBeenLastCalledWith({ page: 1, limit: 10, search: 'SKU_%' }, expect.any(AbortSignal)));
  });

  it('validates Supervisor creation, sends only SKU/name and invalidates the product cache', async () => {
    const list = vi.spyOn(productsApi, 'list').mockResolvedValueOnce({ products: [], meta: { ...meta, total_data: 0, total_pages: 0 } }).mockResolvedValue({ products: [first], meta });
    const create = vi.spyOn(productsApi, 'create').mockResolvedValue(first);
    view(<ProductsPage />, 'SUPERVISOR');
    await userEvent.click(screen.getByRole('button', { name: 'New product' }));
    await userEvent.click(screen.getByRole('button', { name: 'Create product' }));
    expect(screen.getByText('Enter a SKU.')).toBeTruthy();
    expect(create).not.toHaveBeenCalled();
    await userEvent.type(screen.getByLabelText('SKU'), ' SKU-A ');
    await userEvent.type(screen.getByLabelText('Product name'), ' Carton ');
    expect(screen.queryByText('Enter a SKU.')).toBeNull();
    expect(screen.queryByText('Enter a product name.')).toBeNull();
    await userEvent.click(screen.getByRole('button', { name: 'Create product' }));
    await waitFor(() => expect(create).toHaveBeenCalledWith({ sku: 'SKU-A', name: 'Carton' }, expect.anything()));
    expect(await screen.findByText('Product SKU-A created.')).toBeTruthy();
    expect(await screen.findByText('SKU-A')).toBeTruthy();
    expect(list.mock.calls.length).toBeGreaterThan(1);
  });

  it('preserves form input and shows the server duplicate SKU message', async () => {
    catalogue();
    vi.spyOn(productsApi, 'create').mockRejectedValue(new ApiError(409, 'conflict', 'sku already exists'));
    view(<ProductsPage />, 'SUPERVISOR');
    await userEvent.click(screen.getByRole('button', { name: 'New product' }));
    await userEvent.type(screen.getByLabelText('SKU'), 'SKU-A');
    await userEvent.type(screen.getByLabelText('Product name'), 'Carton');
    await userEvent.click(screen.getByRole('button', { name: 'Create product' }));
    expect((await screen.findByRole('alert')).textContent).toContain('sku already exists');
    expect(screen.getByLabelText('SKU')).toHaveProperty('value', 'SKU-A');
  });

  it('renders loading, empty and forbidden error states with retry', async () => {
    const list = vi.spyOn(productsApi, 'list').mockReturnValueOnce(new Promise(() => {}));
    const cache = view(<ProductsPage />);
    expect(screen.getByRole('status').textContent).toContain('Loading products');
    list.mockResolvedValueOnce({ products: [], meta: { ...meta, total_data: 0, total_pages: 0 } });
    await cache.cancelQueries();
    await cache.refetchQueries();
    expect(await screen.findByText('No products on this page')).toBeTruthy();
    list.mockRejectedValueOnce(new ApiError(403, 'forbidden', 'Permission denied'));
    await cache.refetchQueries();
    expect(await screen.findByText('Access denied')).toBeTruthy();
    list.mockResolvedValue({ products: [first], meta });
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }));
    expect(await screen.findByText('SKU-A')).toBeTruthy();
  });
});

describe('receiving documents', () => {
  it('lists server statuses and applies only documented status/pagination filters', async () => {
    const list = vi.spyOn(receivingApi, 'list').mockResolvedValue({ receivings: [document], meta });
    view(<ReceivingListPage />);
    expect(await screen.findByRole('link', { name: 'ORD-24' })).toHaveProperty('href', expect.stringContaining(firstID));
    expect(screen.getByRole('link', { name: 'New receiving document' })).toBeTruthy();
    await userEvent.selectOptions(screen.getByLabelText('Status'), 'COMPLETED');
    await waitFor(() => expect(list).toHaveBeenLastCalledWith({ page: 1, limit: 10, status: 'COMPLETED' }, expect.any(AbortSignal)));
  });

  it('denies Supervisor creation routes and hides list create actions', async () => {
    vi.spyOn(receivingApi, 'list').mockResolvedValue({ receivings: [], meta: { ...meta, total_pages: 0, total_data: 0 } });
    view(<ReceivingListPage />, 'SUPERVISOR');
    expect(await screen.findByText('No receiving documents on this page')).toBeTruthy();
    expect(screen.queryByRole('link', { name: 'New receiving document' })).toBeNull();
    cleanup();
    view(<Routes><Route element={<ProtectedRoute roles={['OPERATOR']} />}><Route path="/receivings/new" element={<h1>Protected form</h1>} /></Route></Routes>, 'SUPERVISOR', '/receivings/new');
    expect(screen.getByText('Operator access required')).toBeTruthy();
    expect(screen.queryByText('Protected form')).toBeNull();
  });

  it('renders expected/approved/remaining values from detail data without deriving fake totals', async () => {
    vi.spyOn(receivingApi, 'detail').mockResolvedValue(document);
    view(<Routes><Route path="/receivings/:id" element={<ReceivingDetailPage />} /></Routes>, 'OPERATOR', `/receivings/${firstID}`);
    const table = await screen.findByRole('table');
    expect(within(table).getByText('7')).toBeTruthy();
    expect(within(table).getByText('5')).toBeTruthy();
    expect(within(table).getByText('2')).toBeTruthy();
    expect(screen.getByText('Incomplete')).toBeTruthy();
  });

  it('rejects malformed IDs before fetching and shows a missing-document response', async () => {
    const detail = vi.spyOn(receivingApi, 'detail').mockRejectedValue(new ApiError(404, 'not_found', 'record not found'));
    const node = <Routes><Route path="/receivings/:id" element={<ReceivingDetailPage />} /></Routes>;
    view(node, 'OPERATOR', '/receivings/not-a-uuid');
    expect(screen.getByText('Invalid document identifier')).toBeTruthy();
    expect(detail).not.toHaveBeenCalled();
    cleanup();
    view(node, 'SUPERVISOR', `/receivings/${firstID}`);
    expect(await screen.findByText('Document not found')).toBeTruthy();
  });

  it('validates required/quantity fields, prevents duplicate SKU choices and submits exact multiple lines', async () => {
    catalogue();
    const create = vi.spyOn(receivingApi, 'create').mockResolvedValue(document);
    view(<Routes><Route path="/" element={<CreateReceivingPage />} /><Route path="/receivings/:id" element={<h1>Created detail</h1>} /></Routes>);
    await userEvent.click(screen.getByRole('button', { name: 'Create receiving document' }));
    expect(screen.getByText('Enter the external ORD reference.')).toBeTruthy();
    expect(screen.getByText('Choose a product for this line.')).toBeTruthy();
    expect(create).not.toHaveBeenCalled();
    await userEvent.type(screen.getByLabelText('External ORD reference'), ' ORD-24 ');
    await line(1, firstID, '1.5');
    await userEvent.click(screen.getByRole('button', { name: 'Create receiving document' }));
    expect(screen.getByText('Enter a whole number from 1 to 9223372036854775807.')).toBeTruthy();
    await userEvent.clear(screen.getByLabelText('Expected quantity for line 1'));
    await userEvent.type(screen.getByLabelText('Expected quantity for line 1'), '7');
    expect(screen.queryByText('Enter a whole number from 1 to 9223372036854775807.')).toBeNull();
    await userEvent.click(screen.getByRole('button', { name: 'Add SKU line' }));
    expect(within(screen.getByLabelText('Product for line 2')).getByRole('option', { name: 'SKU-A — Carton' })).toHaveProperty('disabled', true);
    await line(2, secondID, '9223372036854775807');
    await userEvent.click(screen.getByRole('button', { name: 'Add SKU line' }));
    await waitFor(() => expect(screen.getByLabelText('Product for line 3')).toBe(globalThis.document.activeElement));
    await userEvent.click(screen.getByRole('button', { name: 'Remove SKU line 3' }));
    await waitFor(() => expect(screen.getByLabelText('Product for line 2')).toBe(globalThis.document.activeElement));
    await userEvent.click(screen.getByRole('button', { name: 'Create receiving document' }));
    expect(await screen.findByText('Created detail')).toBeTruthy();
    expect(create).toHaveBeenCalledWith({ order_reference: 'ORD-24', items: [{ product_id: firstID, expected_quantity: 7n }, { product_id: secondID, expected_quantity: 9223372036854775807n }] }, expect.anything());
  });

  it('keeps selected products across catalogue pages and retains inputs on duplicate ORD errors', async () => {
    const list = vi.spyOn(productsApi, 'list').mockImplementation(async filters => ({ products: filters.page === 1 ? [first] : [second], meta: { ...meta, current_page: filters.page, total_pages: 2, total_data: 11 } }));
    vi.spyOn(receivingApi, 'create').mockRejectedValue(new ApiError(409, 'conflict', 'order reference already exists'));
    view(<CreateReceivingPage />);
    await userEvent.type(screen.getByLabelText('External ORD reference'), 'ORD-24');
    await line(1, firstID, '2');
    await userEvent.click(screen.getByRole('button', { name: 'Next' }));
    await waitFor(() => expect(list).toHaveBeenLastCalledWith({ page: 2, limit: 10, search: '' }, expect.any(AbortSignal)));
    expect(screen.getByLabelText('Product for line 1')).toHaveProperty('value', firstID);
    await userEvent.click(screen.getByRole('button', { name: 'Create receiving document' }));
    expect((await screen.findByRole('alert')).textContent).toContain('order reference already exists');
    expect(screen.getByLabelText('External ORD reference')).toHaveProperty('value', 'ORD-24');
    expect(screen.getByLabelText('Expected quantity for line 1')).toHaveProperty('value', '2');
  });

  it('accepts only canonical v4 IDs and exact positive int64 quantities', () => {
    expect(isCanonicalID(firstID)).toBe(true);
    for (const id of [firstID + '=', '550e8400-e29b-41d4-a716-446655440000', 'AAAAAAAAAAAAAAAAAAAAAA', firstID.slice(0, -1) + 'B']) expect(isCanonicalID(id)).toBe(false);
    for (const quantity of ['', '0', '-1', '1.1', '1e3', 'NaN', '9223372036854775808']) expect(positiveQuantity(quantity)).toBeNull();
    expect(positiveQuantity('9223372036854775807')).toBe(9223372036854775807n);
  });
});

describe('locked wire compatibility', () => {
  it('preserves int64 JSON quantities as numeric tokens and exact response values', async () => {
    const fetcher = vi.fn<typeof fetch>(async (_url, options) => {
      expect(options?.body).toBe('{"order_reference":"ORD-24","items":[{"product_id":"VQ6EAOKbQdSnFkRmVUQAAA","expected_quantity":9223372036854775807}]}');
      return new Response('{"success":true,"data":{"expected_quantity":9223372036854775807,"approved_quantity":0,"remaining_quantity":9223372036854775807}}', { headers: { 'Content-Type': 'application/json' } });
    });
    const client = new ApiClient('/api/v1', fetcher);
    expect(await client.request('/receivings', { method: 'POST', body: { order_reference: 'ORD-24', items: [{ product_id: firstID, expected_quantity: 9223372036854775807n }] } })).toEqual({ expected_quantity: 9223372036854775807n, approved_quantity: 0n, remaining_quantity: 9223372036854775807n });
    expect(JSON.parse(encodeJSON({ name: 'quoted " text', items: [1n, 2n] }))).toEqual({ name: 'quoted " text', items: [1, 2] });
    expect(decodeJSON('{"expected_quantity":9007199254740993}')).toEqual({ expected_quantity: 9007199254740993n });
  });

  it('sends exact endpoint paths, supported filters and no actor/status request fields', async () => {
    const request = vi.spyOn(api, 'request').mockResolvedValue({});
    await productsApi.list({ page: 2, limit: 25, search: 'A_%' });
    expect(request).toHaveBeenLastCalledWith('/products?page=2&limit=25&search=A_%25', { signal: undefined });
    await receivingApi.list({ page: 1, limit: 10, status: 'INCOMPLETE' });
    expect(request).toHaveBeenLastCalledWith('/receivings?page=1&limit=10&status=INCOMPLETE', { signal: undefined });
    await receivingApi.create({ order_reference: 'ORD-24', items: [{ product_id: firstID, expected_quantity: 7n }] });
    expect(request).toHaveBeenLastCalledWith('/receivings', { method: 'POST', body: { order_reference: 'ORD-24', items: [{ product_id: firstID, expected_quantity: 7n }] } });
  });
});
