import { randomUUID } from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';
import { test, expect, type Page, type APIRequestContext, type APIResponse } from '@playwright/test';
import { decodeJSON } from '../src/lib/api/json';
import type { LoginData, Role } from '../src/lib/api/types';
import type { Notification, NotificationsPage } from '../src/features/notifications/api';
import type { BalancesPage } from '../src/features/inventory/api';

test.setTimeout(90_000);
function required(key: string) { const value = process.env[key]; if (!value) throw new Error(`Set ${key}`); return value; }
async function login(page: Page, role: Role) {
  await page.goto('/login');
  await page.getByLabel('Email address').fill(required(`E2E_${role}_EMAIL`));
  await page.getByLabel('Password', { exact: true }).fill(required(`E2E_${role}_PASSWORD`));
  const response = page.waitForResponse(response => response.url().endsWith('/auth/login'));
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  expect((await response).status()).toBe(200);
  const auth = (await (await response).json()).data as LoginData;
  await expect(page.getByRole('heading', { name: 'Your warehouse workspace.' })).toBeVisible();
  return auth;
}
async function evidence(response: APIResponse, method = 'GET') {
  if (!process.env.E2E_CONTRACT_DIR) return;
  await mkdir(process.env.E2E_CONTRACT_DIR, { recursive: true });
  await writeFile(`${process.env.E2E_CONTRACT_DIR}/${randomUUID()}.json`, JSON.stringify([{ path: new URL(response.url()).pathname.replace('/api/v1', ''), method, status: response.status(), body_text: await response.text() }]));
}
async function notifications(request: APIRequestContext, token: string) {
  const response = await request.get('/api/v1/notifications?limit=100', { headers: { Authorization: `Bearer ${token}` } });
  expect(response.status()).toBe(200); await evidence(response);
  return (await response.json()).data as NotificationsPage;
}
for (const role of ['OPERATOR', 'SUPERVISOR'] as const) test(`${role} inventory, own notifications, exact quantities and real dashboard totals`, async ({ page, context }) => {
  const auth = await login(page, role);
  const headers = { Authorization: `Bearer ${auth.access_token}` };
  const balanceResponse = await context.request.get('/api/v1/inventory?page=1&limit=10', { headers });
  expect(balanceResponse.status()).toBe(200);
  const balances = (decodeJSON(await balanceResponse.text()) as { data: BalancesPage }).data;
  await page.goto('/inventory');
  if (balances.balances.length) {
    const first = balances.balances[0]!;
    const row = page.getByRole('row').filter({ hasText: first.sku });
    await expect(row.getByText(first.quantity.toLocaleString(), { exact: true })).toBeVisible();
    await row.getByRole('link', { name: `View history for ${first.sku}` }).click();
    await expect(page.getByRole('table', { name: 'Approved inbound ledger' })).toBeVisible();
    await expect(page.getByRole('table').getByText(first.sku).first()).toBeVisible();
  } else await expect(page.getByText('No inventory on this page')).toBeVisible();
  const mine = await notifications(context.request, auth.access_token);
  expect(mine.notifications.every(item => item.recipient_id === auth.user.id)).toBe(true);
  expect(mine.notifications.length).toBeGreaterThan(0);
  await page.goto('/notifications?limit=100');
  const unread = mine.notifications.find(item => !item.read_at);
  if (unread) {
    const row = page.locator('.notification-list li').filter({ has: page.locator(`a[href="/transactions/${unread.receiving_transaction_id}"]`) }).filter({ hasText: unread.message }).first();
    const patch = page.waitForResponse(response => response.url().endsWith(`/notifications/${unread.id}/read`));
    await row.getByRole('button', { name: `Mark as read: ${unread.message}` }).click();
    const saved = await patch; expect(saved.status()).toBe(200);
    expect(((await saved.json()).data as Notification).read_at).toBeTruthy();
    await expect(row.getByText('Read', { exact: true })).toBeVisible();
  }
  await page.goto('/');
  const summary = page.getByRole('region', { name: 'Warehouse summary' });
  const incomplete = await context.request.get('/api/v1/receivings?status=INCOMPLETE&limit=1', { headers });
  const total = (await incomplete.json()).data.meta.total_data as number;
  await expect(summary.locator('.summary-card').filter({ hasText: 'Incomplete receivings' }).locator('.summary-value')).toHaveText(total.toLocaleString());
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  const refreshed = page.waitForResponse(response => response.url().endsWith('/auth/refresh'));
  await page.reload(); expect((await refreshed).status()).toBe(200);
  expect((await context.cookies()).filter(cookie => ['refresh_token', 'session_id'].includes(cookie.name)).every(cookie => cookie.secure && cookie.httpOnly && cookie.sameSite === 'Strict')).toBe(true);
  await page.getByRole('button', { name: 'Open navigation' }).click();
  const logout = page.waitForResponse(response => response.url().endsWith('/auth/logout'));
  await page.getByRole('dialog', { name: 'Navigation' }).getByRole('button', { name: 'Sign out', exact: true }).click();
  expect((await logout).status()).toBe(204);
  await expect(page).toHaveURL(/\/login$/);
  await page.goto('/notifications'); await expect(page).toHaveURL(/\/login$/);
});

test('mobile inventory confines accessible labels to the scrolling table', async ({ page, context }) => {
  const auth = await login(page, 'OPERATOR');
  try {
    const response = await context.request.get('/api/v1/inventory?limit=1', { headers: { Authorization: `Bearer ${auth.access_token}` } });
    expect(response.status()).toBe(200);
    const balances = (decodeJSON(await response.text()) as { data: BalancesPage }).data;
    expect(balances.balances.length).toBeGreaterThan(0);
    const first = balances.balances[0]!;
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`/inventory?search=${encodeURIComponent(first.sku)}`);
    const scroll = page.getByRole('region', { name: 'Approved inbound balances', exact: true });
    await expect(scroll).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await scroll.evaluate(element => { element.scrollLeft = element.scrollWidth; });
    await expect(scroll.getByRole('link', { name: `View history for ${first.sku}`, exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  } finally {
    const logout = await context.request.post('/api/v1/auth/logout', { headers: { Authorization: `Bearer ${auth.access_token}` } });
    expect(logout.status()).toBe(204);
  }
});

test('notification recipient isolation across account change', async ({ page, context }) => {
  const operator = await login(page, 'OPERATOR');
  const own = await notifications(context.request, operator.access_token);
  const notice = own.notifications[0]!; expect(notice).toBeTruthy();
  await page.getByRole('button', { name: 'Sign out', exact: true }).click();
  const supervisor = await login(page, 'SUPERVISOR');
  const denied = await context.request.patch(`/api/v1/notifications/${notice.id}/read`, { headers: { Authorization: `Bearer ${supervisor.access_token}` } });
  expect(denied.status()).toBe(404); await evidence(denied, 'PATCH'); expect((await denied.json()).error.code).toBe('not_found');
  const mine = await notifications(context.request, supervisor.access_token);
  expect(mine.notifications.every(item => item.recipient_id === supervisor.user.id)).toBe(true);
  expect(mine.notifications.some(item => item.id === notice.id)).toBe(false);
  await page.goto('/notifications?limit=100');
  await expect(page.getByRole('heading', { name: 'Notifications', exact: true })).toBeVisible();
  await expect(page.locator('.notification-copy > p')).toHaveText(mine.notifications.map(item => item.message));
});

test('approved maximum int64 balance and ledger remain exact in the browser', async ({ page, context, browser }) => {
  const supervisor = await login(page, 'SUPERVISOR');
  const operatorContext = await browser.newContext({ baseURL: required('E2E_BASE_URL') });
  try {
    const operatorLogin = await operatorContext.request.post('/api/v1/auth/login', { data: { email: required('E2E_OPERATOR_EMAIL'), password: required('E2E_OPERATOR_PASSWORD') } });
    expect(operatorLogin.status()).toBe(200);
    const operator = (await operatorLogin.json()).data as LoginData;
    const supervisorHeaders = { Authorization: `Bearer ${supervisor.access_token}`, 'Content-Type': 'application/json' };
    const operatorHeaders = { Authorization: `Bearer ${operator.access_token}`, 'Content-Type': 'application/json' };
    const sku = 'INT64-' + Date.now().toString(36);
    const productResult = await context.request.post('/api/v1/products', { headers: supervisorHeaders, data: { sku, name: 'Exact quantity proof' } });
    expect(productResult.status()).toBe(201); const product = (await productResult.json()).data as { id: string };
    const receivingResult = await operatorContext.request.post('/api/v1/receivings', { headers: operatorHeaders, data: `{"order_reference":"${sku}","items":[{"product_id":"${product.id}","expected_quantity":9223372036854775807}]}` });
    expect(receivingResult.status()).toBe(201); const receiving = (await receivingResult.json()).data as { id: string; items: { id: string }[] };
    const transactionResult = await operatorContext.request.post(`/api/v1/receivings/${receiving.id}/transactions`, { headers: operatorHeaders, data: `{"items":[{"receiving_document_item_id":"${receiving.items[0]!.id}","actual_quantity":9223372036854775807}]}` });
    expect(transactionResult.status()).toBe(201); const transaction = (await transactionResult.json()).data as { id: string; status: string };
    expect(transaction.status).toBe('PENDING_APPROVAL');
    await page.goto(`/transactions/${transaction.id}`);
    await page.getByRole('button', { name: 'Approve transaction', exact: true }).click();
    const approval = page.waitForResponse(response => response.url().endsWith(`/transactions/${transaction.id}/approve`));
    await page.getByRole('dialog').getByRole('button', { name: 'Confirm approval' }).click(); expect((await approval).status()).toBe(200);
    await page.goto(`/inventory?search=${sku}`);
    await expect(page.getByRole('table').getByText('9,223,372,036,854,775,807', { exact: true })).toBeVisible();
    await page.getByRole('link', { name: `View history for ${sku}` }).click();
    await expect(page.getByRole('table').getByText('+9,223,372,036,854,775,807', { exact: true })).toBeVisible();
    await expect(page.getByRole('table').getByText(sku, { exact: true })).toBeVisible();
    await evidence(await context.request.get(`/api/v1/inventory?search=${sku}`, { headers: supervisorHeaders }));
    await evidence(await context.request.get(`/api/v1/inventory/transactions?product_id=${product.id}`, { headers: supervisorHeaders }));
  } finally { await operatorContext.close(); }
});
