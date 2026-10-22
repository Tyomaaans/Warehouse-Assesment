import { randomUUID } from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';
import { test, expect, type Page, type Browser, type BrowserContext, type APIResponse, type Response } from '@playwright/test';
import { decodeJSON } from '../src/lib/api/json';
import type { LoginData, Role } from '../src/lib/api/types';
import type { ReceivingDetail } from '../src/features/receiving/api';
import type { Transaction } from '../src/features/transactions/api';

test.setTimeout(90_000);
const records: { path: string; method: string; status: number; body_text: string; request_text?: string }[] = [];
function required(name: string) { const value = process.env[name]; if (!value) throw new Error(`Set ${name} for real backend tests.`); return value; }
async function capture<T>(response: APIResponse | Response, method = 'GET', requestText?: string): Promise<T> {
  const text = await response.text();
  records.push({ path: new URL(response.url()).pathname.replace('/api/v1', ''), method, status: response.status(), body_text: text, request_text: requestText });
  return (decodeJSON(text) as { data: T }).data;
}
test.afterEach(async () => {
  const evidence = records.splice(0);
  if (process.env.E2E_CONTRACT_DIR) { await mkdir(process.env.E2E_CONTRACT_DIR, { recursive: true }); await writeFile(`${process.env.E2E_CONTRACT_DIR}/${randomUUID()}.json`, JSON.stringify(evidence)); }
});
async function login(page: Page, role: Role) {
  await page.goto('/login');
  await page.getByLabel('Email address').fill(required(`E2E_${role}_EMAIL`));
  await page.getByLabel('Password', { exact: true }).fill(required(`E2E_${role}_PASSWORD`));
  const response = page.waitForResponse(response => response.url().endsWith('/api/v1/auth/login') && response.request().method() === 'POST');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  const result = await response; expect(result.status()).toBe(200);
  await expect(page.getByRole('button', { name: 'Sign out', exact: true })).toBeVisible();
  return (await result.json()).data as LoginData;
}
interface Actor { page: Page; context: BrowserContext; auth: LoginData }
async function actors(browser: Browser, page: Page, context: BrowserContext) {
  const operator = { page, context, auth: await login(page, 'OPERATOR') };
  const supervisorContext = await browser.newContext({ baseURL: required('E2E_BASE_URL') });
  const supervisorPage = await supervisorContext.newPage();
  const supervisor = { page: supervisorPage, context: supervisorContext, auth: await login(supervisorPage, 'SUPERVISOR') };
  return { operator, supervisor };
}
const headers = (actor: Actor) => ({ Authorization: `Bearer ${actor.auth.access_token}` });
async function fixture(operator: Actor, supervisor: Actor, quantities: number[]) {
  const prefix = `TRX-UI-${randomUUID().slice(0, 8)}`;
  const products: { id: string; sku: string }[] = [];
  for (let index = 0; index < quantities.length; index++) {
    const body = { sku: `${prefix}-${index === 0 ? 'A' : 'B'}`, name: index === 0 ? 'Cartons' : 'Pallets' };
    const result = await supervisor.context.request.post('/api/v1/products', { headers: headers(supervisor), data: body });
    expect(result.status()).toBe(201); products.push(await capture(result, 'POST', JSON.stringify(body)));
  }
  const body = { order_reference: `${prefix}-ORD`, items: products.map((product, index) => ({ product_id: product.id, expected_quantity: quantities[index] })) };
  const result = await operator.context.request.post('/api/v1/receivings', { headers: headers(operator), data: body });
  expect(result.status()).toBe(201);
  const receiving = await capture<ReceivingDetail>(result, 'POST', JSON.stringify(body));
  return { receiving, products, prefix };
}
async function record(actor: Actor, receiving: ReceivingDetail, sku: string, quantity: string) {
  await actor.page.goto(`/receivings/${receiving.id}`);
  await actor.page.getByRole('link', { name: 'Record physical receipt', exact: true }).click();
  await actor.page.getByLabel(`Include ${sku}`, { exact: true }).check();
  await actor.page.getByLabel(`Actual quantity for ${sku}`, { exact: true }).fill(quantity);
  const response = actor.page.waitForResponse(response => response.url().endsWith(`/api/v1/receivings/${receiving.id}/transactions`) && response.request().method() === 'POST');
  await actor.page.getByRole('button', { name: 'Record physical receipt', exact: true }).click();
  const result = await response; expect(result.status()).toBe(201);
  const transaction = await capture<Transaction>(result, 'POST', result.request().postData()!);
  await expect(actor.page).toHaveURL(new RegExp(`/transactions/${transaction.id}$`));
  await expect(actor.page.getByRole('status')).toContainText('Physical receipt recorded');
  expect(transaction.receiving_document_id).toBe(receiving.id);
  return transaction;
}
async function approve(actor: Actor, transaction: Transaction, checkCancel = false) {
  await actor.page.goto(`/transactions/${transaction.id}`);
  await actor.page.getByRole('button', { name: 'Approve transaction', exact: true }).click();
  const dialog = actor.page.getByRole('dialog', { name: 'Approve receiving transaction?' });
  await expect(dialog).toBeVisible();
  if (checkCancel) {
    await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeFocused();
    await actor.page.keyboard.press('Escape'); await expect(dialog).toHaveCount(0);
    const unchanged = await actor.context.request.get(`/api/v1/transactions/${transaction.id}`, { headers: headers(actor) });
    expect((await capture<Transaction>(unchanged)).status).toBe('PENDING_APPROVAL');
    await actor.page.getByRole('button', { name: 'Approve transaction', exact: true }).click();
  }
  const response = actor.page.waitForResponse(response => response.url().endsWith(`/api/v1/transactions/${transaction.id}/approve`));
  await dialog.getByRole('button', { name: 'Confirm approval' }).click();
  const result = await response; expect(result.status()).toBe(200);
  const approved = await capture<Transaction>(result, 'POST');
  await expect(actor.page.getByText('Approved', { exact: true })).toBeVisible();
  await expect(actor.page.getByText('Approved transactions are immutable.')).toBeVisible();
  await expect(actor.page.getByRole('button', { name: 'Approve transaction', exact: true })).toHaveCount(0);
  expect(approved.approved_by).toBe(actor.auth.user.id); expect(approved.approved_at).toBeTruthy();
  return approved;
}
async function document(actor: Actor, id: string) {
  const response = await actor.context.request.get(`/api/v1/receivings/${id}`, { headers: headers(actor) }); expect(response.status()).toBe(200);
  return capture<ReceivingDetail>(response);
}
async function stock(actor: Actor, prefix: string, product: { id: string }, expected: number, movements: number) {
  const balancesResponse = await actor.context.request.get(`/api/v1/inventory?search=${prefix}&limit=100`, { headers: headers(actor) });
  const balances = await capture<{ balances: { product_id: string; quantity: number }[] }>(balancesResponse);
  if (expected === 0) expect(balances.balances.find(row => row.product_id === product.id)).toBeUndefined();
  else expect(balances.balances.find(row => row.product_id === product.id)?.quantity).toBe(expected);
  const historyResponse = await actor.context.request.get(`/api/v1/inventory/transactions?product_id=${product.id}&limit=100`, { headers: headers(actor) });
  const history = await capture<{ movements: { quantity_delta: number; receiving_transaction_id: string; receiving_document_id: string }[] }>(historyResponse);
  expect(history.movements).toHaveLength(movements);
  expect(history.movements.reduce((sum, row) => sum + row.quantity_delta, 0)).toBe(expected);
  return history.movements;
}
async function screenshot(page: Page, name: string) { if (process.env.E2E_SCREENSHOT_DIR) { await mkdir(process.env.E2E_SCREENSHOT_DIR, { recursive: true }); await page.screenshot({ path: `${process.env.E2E_SCREENSHOT_DIR}/${name}.png`, fullPage: true }); } }

test('partial approval, persisted excess/correction, genuine excess resolution and all-SKU completion', async ({ page, context, browser }) => {
  const { operator, supervisor } = await actors(browser, page, context);
  try {
    const { receiving, products, prefix } = await fixture(operator, supervisor, [10, 5]);
    const first = await record(operator, receiving, products[0]!.sku, '4');
    expect(first.status).toBe('PENDING_APPROVAL');
    await stock(supervisor, prefix, products[0]!, 0, 0);
    await page.goto(`/receivings/${receiving.id}/transactions/new`);
    await expect(page.getByRole('heading', { name: 'An active transaction already exists' })).toBeVisible();
    const activeAttempt = await operator.context.request.post(`/api/v1/receivings/${receiving.id}/transactions`, { headers: headers(operator), data: { items: [{ receiving_document_item_id: receiving.items[0]!.id, actual_quantity: 1 }] } });
    expect(activeAttempt.status()).toBe(409); await capture(activeAttempt, 'POST');
    const deniedApproval = await operator.context.request.post(`/api/v1/transactions/${first.id}/approve`, { headers: headers(operator) });
    expect(deniedApproval.status()).toBe(403); await capture(deniedApproval, 'POST');
    const deniedSubmission = await supervisor.context.request.post(`/api/v1/receivings/${receiving.id}/transactions`, { headers: headers(supervisor), data: { items: [{ receiving_document_item_id: receiving.items[0]!.id, actual_quantity: 1 }] } });
    expect(deniedSubmission.status()).toBe(403); await capture(deniedSubmission, 'POST');
    await supervisor.page.goto(`/receivings/${receiving.id}/transactions/new`);
    await expect(supervisor.page.getByRole('heading', { name: 'Operator access required' })).toBeVisible();
    await approve(supervisor, first, true);
    await stock(supervisor, prefix, products[0]!, 4, 1);
    const repeated = await supervisor.context.request.post(`/api/v1/transactions/${first.id}/approve`, { headers: headers(supervisor) });
    expect(repeated.status()).toBe(409); await capture(repeated, 'POST');
    await stock(supervisor, prefix, products[0]!, 4, 1);
    const partial = await document(supervisor, receiving.id);
    expect(partial.status).toBe('INCOMPLETE'); expect(partial.items.find(item => item.product_id === products[1]!.id)?.approved_quantity).toBe(0n);
    await expect(supervisor.page.getByText('Incomplete', { exact: true })).toBeVisible();
    const excess = await record(operator, receiving, products[0]!.sku, '60');
    expect(excess.status).toBe('REJECTED_BY_SYSTEM'); expect(excess.review_status).toBe('UNRESOLVED');
    await expect(page.getByRole('status')).toContainText('REJECTED_BY_SYSTEM / UNRESOLVED');
    await expect(page.getByRole('button', { name: 'Correct recorded input' })).toHaveCount(0);
    for (const [path, method, body] of [['correct', 'PATCH', { reason: 'Denied', items: [{ receiving_document_item_id: excess.items[0]!.receiving_document_item_id, actual_quantity: 6 }] }], ['review', 'POST', { note: 'Denied' }]] as const) {
      const denied = await operator.context.request.fetch(`/api/v1/transactions/${excess.id}/${path}`, { method, headers: headers(operator), data: body });
      expect(denied.status()).toBe(403); await capture(denied, method);
    }
    await supervisor.page.goto(`/transactions/${excess.id}`);
    await supervisor.page.getByRole('button', { name: 'Correct recorded input' }).click();
    await expect(supervisor.page.getByText('Recorded before correction').first()).toBeVisible();
    await supervisor.page.getByLabel(`Corrected quantity for ${products[0]!.sku}`).fill('6');
    await supervisor.page.getByRole('button', { name: 'Save correction' }).click();
    await expect(supervisor.page.getByText('Enter the reason for this correction.')).toBeVisible();
    await supervisor.page.getByLabel('Correction reason').fill('Count entry contained an extra zero');
    await screenshot(supervisor.page, 'correction-desktop');
    const correctionResponse = supervisor.page.waitForResponse(response => response.url().endsWith(`/api/v1/transactions/${excess.id}/correct`));
    await supervisor.page.getByRole('button', { name: 'Save correction' }).click();
    const correction = await correctionResponse; expect(correction.status()).toBe(200);
    const corrected = await capture<Transaction>(correction, 'PATCH', correction.request().postData()!);
    expect(corrected.id).toBe(excess.id); expect(corrected.receiving_document_id).toBe(receiving.id); expect(corrected.status).toBe('PENDING_APPROVAL');
    expect(corrected.corrected_by).toBe(supervisor.auth.user.id); expect(corrected.corrected_at).toBeTruthy();
    await expect(supervisor.page.getByText('Pending approval', { exact: true })).toBeVisible();
    await stock(supervisor, prefix, products[0]!, 4, 1);
    const auditResponse = await supervisor.context.request.get(`/api/v1/receiving-audit-logs?receiving_id=${receiving.id}&limit=100`, { headers: headers(supervisor) });
    const audit = await capture<{ audit_logs: { event_type: string; reason: string; actor_id: string; before_data: Transaction; after_data: Transaction }[] }>(auditResponse);
    const changed = audit.audit_logs.find(row => row.event_type === 'TRANSACTION_CORRECTED');
    expect(changed?.reason).toBe('Count entry contained an extra zero'); expect(changed?.actor_id).toBe(supervisor.auth.user.id);
    expect(changed?.before_data.items[0]!.actual_quantity).toBe(60n); expect(changed?.after_data.items[0]!.actual_quantity).toBe(6n);
    await approve(supervisor, corrected); await stock(supervisor, prefix, products[0]!, 10, 2);
    expect((await document(supervisor, receiving.id)).status).toBe('INCOMPLETE');
    const physicalExcess = await record(operator, receiving, products[1]!.sku, '8');
    expect(physicalExcess.status).toBe('REJECTED_BY_SYSTEM');
    await supervisor.page.goto(`/transactions/${physicalExcess.id}`);
    await supervisor.page.getByRole('button', { name: 'Resolve confirmed excess' }).click();
    await supervisor.page.getByRole('button', { name: 'Resolve review' }).click();
    await expect(supervisor.page.getByText('Enter a verification note for the confirmed physical excess.')).toBeVisible();
    await supervisor.page.getByLabel('Verification note').fill('Physically confirmed excess; no stock posted');
    const reviewResponse = supervisor.page.waitForResponse(response => response.url().endsWith(`/api/v1/transactions/${physicalExcess.id}/review`));
    await supervisor.page.getByRole('button', { name: 'Resolve review' }).click();
    const review = await reviewResponse; expect(review.status()).toBe(200);
    const resolved = await capture<Transaction>(review, 'POST', review.request().postData()!);
    expect(resolved.status).toBe('REJECTED_BY_SYSTEM'); expect(resolved.review_status).toBe('RESOLVED');
    expect(resolved.reviewed_by).toBe(supervisor.auth.user.id); expect(resolved.review_note).toBe('Physically confirmed excess; no stock posted');
    await expect(supervisor.page.getByText('Resolved', { exact: true })).toBeVisible();
    await stock(supervisor, prefix, products[1]!, 0, 0);
    const final = await record(operator, receiving, products[1]!.sku, '5');
    expect(final.status).toBe('PENDING_APPROVAL'); await approve(supervisor, final);
    await expect(supervisor.page.getByText('Completed', { exact: true })).toBeVisible();
    const complete = await document(operator, receiving.id); expect(complete.status).toBe('COMPLETED');
    expect(complete.items.every(item => item.approved_quantity === item.expected_quantity && item.remaining_quantity === 0n)).toBe(true);
    const movements = await stock(supervisor, prefix, products[1]!, 5, 1); expect(movements[0]!.receiving_transaction_id).toBe(final.id); expect(movements[0]!.receiving_document_id).toBe(receiving.id);
    await stock(supervisor, prefix, products[0]!, 10, 2);
    await page.goto(`/receivings/${receiving.id}`); await expect(page.getByRole('table', { name: 'Receiving transaction history' })).toBeVisible();
    const historyResponse = await operator.context.request.get(`/api/v1/receivings/${receiving.id}/transactions?page=1&limit=10`, { headers: headers(operator) });
    const history = await capture<{ transactions: Transaction[] }>(historyResponse); expect(history.transactions).toHaveLength(4);
    await expect(page.getByRole('link', { name: 'Record physical receipt', exact: true })).toHaveCount(0);
    await page.setViewportSize({ width: 390, height: 844 }); expect(await page.evaluate(() => globalThis.document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await screenshot(page, 'receiving-history-mobile');
    await page.goto(`/receivings/${receiving.id}/transactions/new`); await expect(page.getByRole('heading', { name: 'Receiving is completed' })).toBeVisible();
  } finally { await supervisor.context.close(); }
});

test('maximum int64 physical excess remains exact through rejection and still-rejected correction', async ({ page, context, browser }) => {
  const { operator, supervisor } = await actors(browser, page, context);
  try {
    const { receiving, products, prefix } = await fixture(operator, supervisor, [1]);
    const excess = await record(operator, receiving, products[0]!.sku, '9223372036854775807');
    expect(excess.status).toBe('REJECTED_BY_SYSTEM'); expect(excess.items[0]!.actual_quantity).toBe(9223372036854775807n);
    await expect(page.getByRole('table', { name: 'Transaction quantities' }).getByText('9,223,372,036,854,775,807', { exact: true })).toBeVisible();
    await supervisor.page.goto(`/transactions/${excess.id}`); await supervisor.page.getByRole('button', { name: 'Correct recorded input' }).click();
    await supervisor.page.getByLabel(`Corrected quantity for ${products[0]!.sku}`).fill('9223372036854775806');
    await supervisor.page.getByLabel('Correction reason').fill('Recount confirms a slightly lower physical excess');
    const response = supervisor.page.waitForResponse(response => response.url().endsWith(`/api/v1/transactions/${excess.id}/correct`));
    await supervisor.page.getByRole('button', { name: 'Save correction' }).click();
    const result = await response; expect(result.status()).toBe(200);
    const corrected = await capture<Transaction>(result, 'PATCH', result.request().postData()!);
    expect(corrected.id).toBe(excess.id); expect(corrected.status).toBe('REJECTED_BY_SYSTEM'); expect(corrected.review_status).toBe('UNRESOLVED');
    expect(corrected.items[0]!.actual_quantity).toBe(9223372036854775806n);
    await expect(supervisor.page.getByRole('status')).toContainText('remains rejected and unresolved');
    await stock(supervisor, prefix, products[0]!, 0, 0);
  } finally { await supervisor.context.close(); }
});

test('approval race returns 409, refreshes immutable state and never double-posts inventory', async ({ page, context, browser }) => {
  const { operator, supervisor } = await actors(browser, page, context);
  try {
    const { receiving, products, prefix } = await fixture(operator, supervisor, [2]);
    const pending = await record(operator, receiving, products[0]!.sku, '2');
    await supervisor.page.goto(`/transactions/${pending.id}`);
    await supervisor.page.getByRole('button', { name: 'Approve transaction' }).click();
    const competing = await supervisor.context.request.post(`/api/v1/transactions/${pending.id}/approve`, { headers: headers(supervisor) });
    expect(competing.status()).toBe(200); await capture(competing, 'POST');
    const response = supervisor.page.waitForResponse(response => response.url().endsWith(`/api/v1/transactions/${pending.id}/approve`));
    await supervisor.page.getByRole('button', { name: 'Confirm approval' }).click();
    const result = await response; expect(result.status()).toBe(409); await capture(result, 'POST');
    await expect(supervisor.page.getByRole('alert')).toContainText(/completed|pending transactions/);
    await expect(supervisor.page.getByText('Approved transactions are immutable.')).toBeVisible();
    await expect(supervisor.page.getByRole('button', { name: 'Approve transaction' })).toHaveCount(0);
    await stock(supervisor, prefix, products[0]!, 2, 1);
    expect((await document(supervisor, receiving.id)).status).toBe('COMPLETED');
    await screenshot(supervisor.page, 'approval-race-desktop');
  } finally { await supervisor.context.close(); }
});
