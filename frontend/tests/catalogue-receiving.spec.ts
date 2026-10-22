import { randomUUID } from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';
import { test, expect, type Page, type APIResponse, type Response } from '@playwright/test';
import type { LoginData, Role } from '../src/lib/api/types';
import type { Product } from '../src/features/products/api';
import { decodeJSON } from '../src/lib/api/json';
import type { ReceivingDetail } from '../src/features/receiving/api';

const prefix = `UI-${randomUUID().slice(0, 8)}`;
let first: Product;
let second: Product;
let savedDocument: ReceivingDetail;
const records: { path: string; method: string; status: number; body_text: string; request_text?: string }[] = [];
function required(name: string) { const value = process.env[name]; if (!value) throw new Error(`Set ${name} for real backend browser integration.`); return value; }
async function login(page: Page, role: Role): Promise<LoginData> {
  await page.goto('/login');
  await page.getByLabel('Email address').fill(required(`E2E_${role}_EMAIL`));
  await page.getByLabel('Password', { exact: true }).fill(required(`E2E_${role}_PASSWORD`));
  const response = page.waitForResponse(response => response.url().endsWith('/api/v1/auth/login') && response.request().method() === 'POST');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  const result = await response;
  expect(result.status()).toBe(200);
  // Login can restore the protected path saved by the router, including after role switching.
  await expect(page.getByRole('button', { name: 'Sign out', exact: true })).toBeVisible();
  await expect(page).not.toHaveURL(/\/login$/);
  return (await result.json()).data as LoginData;
}
async function direct(response: APIResponse | Response, method = 'GET', requestText?: string) {
  records.push({ path: new URL(response.url()).pathname.replace('/api/v1', ''), method, status: response.status(), body_text: await response.text(), request_text: requestText });
  return response;
}
async function logout(page: Page) { await page.getByRole('button', { name: 'Sign out', exact: true }).click(); await expect(page).toHaveURL(/\/login$/); }
async function screenshot(page: Page, name: string) {
  if (!process.env.E2E_SCREENSHOT_DIR) return;
  await mkdir(process.env.E2E_SCREENSHOT_DIR, { recursive: true });
  await page.screenshot({ path: `${process.env.E2E_SCREENSHOT_DIR}/${name}.png`, fullPage: true });
}

test.describe.configure({ mode: 'serial' });
test.afterEach(async () => {
  if (process.env.E2E_CONTRACT_DIR) {
    await mkdir(process.env.E2E_CONTRACT_DIR, { recursive: true });
    await writeFile(`${process.env.E2E_CONTRACT_DIR}/${randomUUID()}.json`, JSON.stringify(records.splice(0)));
  }
});

test('Supervisor creates/searches products, duplicate SKU feedback, pagination and receiving-create denial', async ({ page, context }) => {
  const auth = await login(page, 'SUPERVISOR');
  await page.goto('/products');
  await page.getByRole('button', { name: 'New product' }).click();
  await page.getByRole('button', { name: 'Create product', exact: true }).click();
  await expect(page.getByText('Enter a SKU.')).toBeVisible();
  await expect(page.getByLabel('SKU')).toBeFocused();
  async function create(sku: string, name: string) {
    await page.getByLabel('SKU').fill(sku);
    await page.getByLabel('Product name').fill(name);
    const response = page.waitForResponse(response => response.url().endsWith('/api/v1/products') && response.request().method() === 'POST');
    await page.getByRole('button', { name: 'Create product', exact: true }).click();
    const result = await response;
    expect(result.status()).toBe(201);
    expect(result.request().postDataJSON()).toEqual({ sku, name });
    await direct(result, 'POST', result.request().postData()!);
    const product = (await result.json()).data as Product;
    expect(product.id).toMatch(/^[A-Za-z0-9_-]{21}[AQgw]$/);
    await expect(page.getByRole('status')).toHaveText(`Product ${sku} created.`);
    await expect(page.getByRole('table').getByText(sku, { exact: true })).toBeVisible();
    return product;
  }
  first = await create(`${prefix}-A`, 'Packing cartons');
  await page.getByRole('button', { name: 'New product' }).click();
  await page.getByLabel('SKU').fill(first.sku);
  await page.getByLabel('Product name').fill('Duplicate attempt');
  const duplicate = page.waitForResponse(response => response.request().method() === 'POST' && response.url().endsWith('/api/v1/products'));
  await page.getByRole('button', { name: 'Create product', exact: true }).click();
  const duplicateResult = await duplicate;
  expect(duplicateResult.status()).toBe(409);
  await direct(duplicateResult, 'POST', duplicateResult.request().postData()!);
  await expect(page.getByRole('alert')).toContainText('already exists');
  await expect(page.getByLabel('SKU')).toHaveValue(first.sku);
  second = await create(`${prefix}-B`, 'Stacking pallets');
  for (let index = 0; index < 9; index++) {
    const result = await direct(await context.request.post('/api/v1/products', { headers: { Authorization: `Bearer ${auth.access_token}` }, data: { sku: `${prefix}-EXTRA-${index}`, name: 'Pagination fixture' } }), 'POST');
    expect(result.status()).toBe(201);
  }
  await page.getByLabel('Search products').fill(prefix);
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.getByRole('navigation', { name: 'Pagination' })).toContainText('11 records');
  await page.getByRole('button', { name: 'Next', exact: true }).click();
  await expect(page).toHaveURL(/page=2/);
  await expect(page.getByRole('navigation', { name: 'Pagination' })).toContainText('Page 2 of 2');
  await page.getByRole('button', { name: 'Previous' }).click();
  await page.getByLabel('Search products').fill(first.sku);
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.getByRole('navigation', { name: 'Pagination' })).toContainText('1 record');
  expect((await direct(await context.request.get(`/api/v1/products?page=1&limit=10&search=${encodeURIComponent(prefix)}`, { headers: { Authorization: `Bearer ${auth.access_token}` } }))).status()).toBe(200);
  await screenshot(page, 'products-desktop');
  await page.goto('/receivings');
  await expect(page.getByRole('link', { name: 'New receiving document' })).toHaveCount(0);
  await page.goto('/receivings/new');
  await expect(page.getByRole('heading', { name: 'Operator access required' })).toBeVisible();
  const denied = await direct(await context.request.post('/api/v1/receivings', { headers: { Authorization: `Bearer ${auth.access_token}` }, data: {} }), 'POST', '{}');
  expect(denied.status()).toBe(403);
  expect((await denied.json()).error.code).toBe('forbidden');
});

test('Operator creates multi-SKU snapshots, exact int64 quantities, duplicate ORD feedback and mobile detail', async ({ page, context }) => {
  const auth = await login(page, 'OPERATOR');
  await page.goto('/products');
  await expect(page.getByRole('button', { name: 'New product' })).toHaveCount(0);
  await page.getByLabel('Search products').fill(first.sku);
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.getByRole('table').getByText(first.sku, { exact: true })).toBeVisible();
  const denied = await direct(await context.request.post('/api/v1/products', { headers: { Authorization: `Bearer ${auth.access_token}` }, data: { sku: `${prefix}-DENIED`, name: 'Denied attempt' } }), 'POST');
  expect(denied.status()).toBe(403);
  await page.goto('/receivings');
  await page.getByRole('link', { name: 'New receiving document' }).click();
  await page.getByRole('button', { name: 'Create receiving document', exact: true }).click();
  await expect(page.getByText('Enter the external ORD reference.')).toBeVisible();
  await expect(page.getByLabel('External ORD reference')).toBeFocused();
  await page.getByLabel('External ORD reference').fill(`${prefix}-ORD`);
  await page.getByLabel('Search available products').fill(prefix);
  await page.getByRole('button', { name: 'Search products', exact: true }).click();
  await page.getByRole('button', { name: 'Next', exact: true }).click();
  await expect(page.getByRole('navigation', { name: 'Pagination' })).toContainText('Page 2 of 2');
  await page.getByLabel('Product for line 1').selectOption(first.id);
  for (const value of ['0', '-1', '1.5', '9223372036854775808']) {
    await page.getByLabel('Expected quantity for line 1').fill(value);
    await page.getByRole('button', { name: 'Create receiving document', exact: true }).click();
    await expect(page.getByText('Enter a whole number from 1 to 9223372036854775807.')).toBeVisible();
  }
  await page.getByLabel('Expected quantity for line 1').fill('7');
  await page.getByRole('button', { name: 'Previous' }).click();
  await expect(page.getByLabel('Product for line 1')).toHaveValue(first.id);
  await page.getByRole('button', { name: 'Add SKU line' }).click();
  await expect(page.getByLabel('Product for line 2')).toBeFocused();
  await page.getByLabel('Product for line 2').selectOption(second.id);
  await expect(page.getByLabel('Product for line 2').locator(`option[value="${first.id}"]`)).toBeDisabled();
  await page.getByLabel('Expected quantity for line 2').fill('9223372036854775807');
  await screenshot(page, 'receiving-create-desktop');
  const created = page.waitForResponse(response => response.url().endsWith('/api/v1/receivings') && response.request().method() === 'POST');
  await page.getByRole('button', { name: 'Create receiving document', exact: true }).click();
  const result = await created;
  expect(result.status()).toBe(201);
  await direct(result, 'POST', result.request().postData()!);
  expect(result.request().postData()).toBe(`{"order_reference":"${prefix}-ORD","items":[{"product_id":"${first.id}","expected_quantity":7},{"product_id":"${second.id}","expected_quantity":9223372036854775807}]}`);
  savedDocument = (decodeJSON(await result.text()) as { data: ReceivingDetail }).data;
  expect(savedDocument.status).toBe('INCOMPLETE');
  expect(savedDocument.items.map(item => ({ product_id: item.product_id, expected: item.expected_quantity, approved: item.approved_quantity, remaining: item.remaining_quantity }))).toEqual(expect.arrayContaining([
    { product_id: first.id, expected: 7n, approved: 0n, remaining: 7n },
    { product_id: second.id, expected: 9223372036854775807n, approved: 0n, remaining: 9223372036854775807n },
  ]));
  await expect(page).toHaveURL(new RegExp(`/receivings/${savedDocument.id}$`));
  await expect(page.getByRole('status').filter({ hasText: 'Receiving document created.' })).toHaveText('Receiving document created.');
  await expect(page.getByText('Incomplete', { exact: true })).toBeVisible();
  const detailResponse = page.waitForResponse(response => response.url().endsWith(`/api/v1/receivings/${savedDocument.id}`));
  await page.reload();
  expect((await direct(await detailResponse)).status()).toBe(200);
  await expect(page.getByRole('table').getByText('9,223,372,036,854,775,807', { exact: true })).toHaveCount(2);
  await screenshot(page, 'receiving-detail-desktop');
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await screenshot(page, 'receiving-detail-mobile');
  await page.goto('/receivings/new');
  await page.getByLabel('External ORD reference').fill(savedDocument.order_reference);
  await page.getByLabel('Search available products').fill(first.sku);
  await page.getByRole('button', { name: 'Search products', exact: true }).click();
  await page.getByLabel('Product for line 1').selectOption(first.id);
  await page.getByLabel('Expected quantity for line 1').fill('2');
  const duplicate = page.waitForResponse(response => response.url().endsWith('/api/v1/receivings') && response.request().method() === 'POST');
  await page.getByRole('button', { name: 'Create receiving document', exact: true }).click();
  const duplicateResult = await duplicate;
  expect(duplicateResult.status()).toBe(409);
  await direct(duplicateResult, 'POST', duplicateResult.request().postData()!);
  await expect(page.getByRole('alert')).toContainText('already exists');
  await expect(page.getByLabel('External ORD reference')).toHaveValue(savedDocument.order_reference);
  await expect(page.getByLabel('Expected quantity for line 1')).toHaveValue('2');
});

test('both roles read paginated documents and exact detail; invalid IDs and missing documents are handled', async ({ page, context }) => {
  const auth = await login(page, 'OPERATOR');
  for (let index = 0; index < 10; index++) {
    const result = await direct(await context.request.post('/api/v1/receivings', { headers: { Authorization: `Bearer ${auth.access_token}` }, data: { order_reference: `${prefix}-PAGE-${index}`, items: [{ product_id: first.id, expected_quantity: 2 }] } }), 'POST');
    expect(result.status()).toBe(201);
  }
  await page.goto('/receivings');
  await page.getByRole('button', { name: 'Next', exact: true }).click();
  await expect(page).toHaveURL(/page=2/);
  await expect(page.getByRole('navigation', { name: 'Pagination' })).toContainText('Page 2');
  const completedResponse = page.waitForResponse(response => new URL(response.url()).pathname === '/api/v1/receivings' && new URL(response.url()).searchParams.get('status') === 'COMPLETED');
  await page.getByRole('combobox', { name: 'Status', exact: true }).selectOption('COMPLETED');
  const completed = await completedResponse;
  const completedData = (await completed.json()).data as { receivings: { status: string }[] };
  await direct(completed);
  expect(completedData.receivings.every(document => document.status === 'COMPLETED')).toBe(true);
  if (completedData.receivings.length) await expect(page.getByRole('table').getByText('Completed', { exact: true })).toHaveCount(completedData.receivings.length);
  else await expect(page.getByText('No receiving documents on this page')).toBeVisible();
  await page.getByRole('combobox', { name: 'Status', exact: true }).selectOption('INCOMPLETE');
  await expect(page.getByRole('table')).toBeVisible();
  expect((await direct(await context.request.get('/api/v1/receivings?page=1&limit=10&status=INCOMPLETE', { headers: { Authorization: `Bearer ${auth.access_token}` } }))).status()).toBe(200);
  await page.goto('/receivings/not-a-canonical-id');
  await expect(page.getByRole('heading', { name: 'Invalid document identifier' })).toBeVisible();
  const invalid = await direct(await context.request.get('/api/v1/receivings/not-a-canonical-id', { headers: { Authorization: `Bearer ${auth.access_token}` } }));
  expect(invalid.status()).toBe(400);
  expect((await invalid.json()).error.code).toBe('invalid_input');
  const missing = Buffer.from(randomUUID().replaceAll('-', ''), 'hex').toString('base64url');
  const missingResponse = page.waitForResponse(response => response.url().endsWith(`/api/v1/receivings/${missing}`));
  await page.goto(`/receivings/${missing}`);
  expect((await direct(await missingResponse)).status()).toBe(404);
  await expect(page.getByRole('heading', { name: 'Document not found' })).toBeVisible();
  await logout(page);
  await login(page, 'SUPERVISOR');
  await page.goto('/receivings');
  await expect(page.getByRole('table')).toBeVisible();
  await expect(page.getByRole('link', { name: 'New receiving document' })).toHaveCount(0);
  await page.goto(`/receivings/${savedDocument.id}`);
  await expect(page.getByRole('heading', { name: savedDocument.order_reference })).toBeVisible();
  await expect(page.getByRole('table').getByText('9,223,372,036,854,775,807', { exact: true })).toHaveCount(2);
});
