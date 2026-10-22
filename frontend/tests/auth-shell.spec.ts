import { createHmac } from 'node:crypto';
import { mkdir } from 'node:fs/promises';
import { test, expect, type Page } from '@playwright/test';
import type { LoginData, Role } from '../src/lib/api/types';

function required(name: string) {
  const value = process.env[name];
  if (!value) throw new Error(`Set ${name} for real backend browser integration.`);
  return value;
}
async function login(page: Page, role: Role, remember = false): Promise<LoginData> {
  await page.goto('/login');
  await page.getByLabel('Email address').fill(required(`E2E_${role}_EMAIL`));
  await page.getByLabel('Password', { exact: true }).fill(required(`E2E_${role}_PASSWORD`));
  if (remember) await page.getByLabel('Keep me signed in').check();
  const response = page.waitForResponse(response => response.url().endsWith('/api/v1/auth/login') && response.request().method() === 'POST');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  const result = await response;
  expect(result.status()).toBe(200);
  await expect(page.getByRole('heading', { name: 'Your warehouse workspace.' })).toBeVisible();
  expect(await page.evaluate(() => window.scrollY)).toBe(0);
  return (await result.json()).data as LoginData;
}
async function screenshot(page: Page, name: string) {
  if (!process.env.E2E_SCREENSHOT_DIR) return;
  await mkdir(process.env.E2E_SCREENSHOT_DIR, { recursive: true });
  await page.screenshot({ path: `${process.env.E2E_SCREENSHOT_DIR}/${name}.png`, fullPage: true });
}

test('anonymous protection, local validation and real invalid-credential feedback', async ({ page }) => {
  await page.goto('/inventory');
  await expect(page).toHaveURL(/\/login$/);
  await screenshot(page, 'login-desktop');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await expect(page.getByText('Enter your email address.')).toBeVisible();
  await expect(page.getByLabel('Email address')).toBeFocused();
  await page.getByLabel('Email address').fill(required('E2E_OPERATOR_EMAIL'));
  await page.getByLabel('Password', { exact: true }).fill('deliberately-incorrect-test-input');
  const response = page.waitForResponse(response => response.url().endsWith('/api/v1/auth/login'));
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  expect((await response).status()).toBe(401);
  await expect(page.getByRole('alert')).toContainText('invalid email or password');
});

test('Operator login, strict cookies, reload rotation, role guard and mobile navigation', async ({ page, context }) => {
  await login(page, 'OPERATOR', true);
  await expect(page.getByRole('navigation', { name: 'Supervisor navigation' })).toHaveCount(0);
  const before = (await context.cookies()).filter(cookie => ['refresh_token', 'session_id'].includes(cookie.name));
  expect(before.map(cookie => ({ name: cookie.name, path: cookie.path, httpOnly: cookie.httpOnly, sameSite: cookie.sameSite }))).toEqual(expect.arrayContaining([
    { name: 'refresh_token', path: '/api/v1/auth', httpOnly: true, sameSite: 'Strict' },
    { name: 'session_id', path: '/api/v1', httpOnly: true, sameSite: 'Strict' },
  ]));
  expect(await page.evaluate(() => ({ local: localStorage.length, session: sessionStorage.length, cookies: document.cookie }))).toEqual({ local: 0, session: 0, cookies: '' });
  await screenshot(page, 'overview-desktop');
  const refreshed = page.waitForResponse(response => response.url().endsWith('/api/v1/auth/refresh'));
  await page.reload();
  expect((await refreshed).status()).toBe(200);
  await expect(page.getByRole('heading', { name: 'Your warehouse workspace.' })).toBeVisible();
  const after = await context.cookies();
  expect(after.find(cookie => cookie.name === 'refresh_token')?.value !== before.find(cookie => cookie.name === 'refresh_token')?.value).toBe(true);
  expect(after.find(cookie => cookie.name === 'session_id')?.value === before.find(cookie => cookie.name === 'session_id')?.value).toBe(true);
  await page.goto('/transactions/review');
  await expect(page.getByRole('heading', { name: 'Supervisor access required' })).toBeVisible();
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Your warehouse workspace.' })).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await screenshot(page, 'overview-mobile');
  await page.getByRole('button', { name: 'Open navigation' }).click();
  await expect(page.getByRole('dialog', { name: 'Navigation' })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toBeHidden();
  await expect(page.getByRole('button', { name: 'Open navigation' })).toBeFocused();
  await page.getByRole('button', { name: 'Open navigation' }).click();
  await page.getByRole('dialog').getByRole('link', { name: 'Notifications', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Notifications', exact: true })).toBeVisible();
  await expect(page.getByText('Planned view', { exact: true })).toBeVisible();
  await expect(page.getByRole('dialog')).toBeHidden();
  await expect(page.locator('#main-content')).toBeFocused();
  expect(await page.evaluate(() => window.scrollY)).toBe(0);
});

test('Supervisor navigation, explicit placeholders, logout 204 and cookie clearing', async ({ page, context }) => {
  await login(page, 'SUPERVISOR');
  await page.getByRole('navigation', { name: 'Supervisor navigation' }).getByRole('link', { name: 'Supervisor review' }).click();
  await expect(page.getByRole('heading', { name: 'Supervisor review', exact: true })).toBeVisible();
  for (const [path, title] of [['/products', 'Products'], ['/receivings', 'Receiving documents'], ['/transactions', 'Receiving transactions']]) {
    await page.goto(path!);
    await expect(page.getByRole('heading', { name: title!, exact: true })).toBeVisible();
    await expect(page.getByText('Planned view', { exact: true })).toHaveCount(0);
  }
  for (const [path, title] of [['/inventory', 'Inventory'], ['/notifications', 'Notifications']]) {
    await page.goto(path!);
    await expect(page.getByRole('heading', { name: title!, exact: true })).toBeVisible();
    await expect(page.getByText('Planned view', { exact: true })).toBeVisible();
    await expect(page.getByText(/no live records are displayed/)).toBeVisible();
  }
  const response = page.waitForResponse(response => response.url().endsWith('/api/v1/auth/logout'));
  await page.getByRole('button', { name: 'Sign out', exact: true }).click();
  expect((await response).status()).toBe(204);
  await expect(page).toHaveURL(/\/login$/);
  expect((await context.cookies()).some(cookie => ['session_id', 'refresh_token'].includes(cookie.name))).toBe(false);
  await page.goto('/inventory');
  await expect(page).toHaveURL(/\/login$/);
});

test('real expired JWTs share refresh; backend role denial and session revocation clear cached state', async ({ page, context }) => {
  const data = await login(page, 'OPERATOR');
  const cookie = (await context.cookies()).find(cookie => cookie.name === 'session_id');
  expect(Boolean(cookie)).toBe(true);
  const internalID = (wire: string) => {
    const hex = Buffer.from(wire, 'base64url').toString('hex');
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
  };
  const encode = (value: unknown) => Buffer.from(JSON.stringify(value)).toString('base64url');
  const now = Math.floor(Date.now() / 1000);
  const unsigned = `${encode({ alg: 'HS256', typ: 'JWT' })}.${encode({ sub: internalID(data.user.id), sid: internalID(cookie!.value), role: 'OPERATOR', iat: now - 1800, exp: now - 900 })}`;
  const expired = `${unsigned}.${createHmac('sha256', required('E2E_JWT_SECRET')).update(unsigned).digest('base64url')}`;
  let refreshes = 0;
  page.on('request', request => { if (request.url().endsWith('/api/v1/auth/refresh')) refreshes++; });
  const result = await page.evaluate(async token => {
    const path = '/src/lib/api/client.ts';
    const { ApiClient } = await import(path) as typeof import('../src/lib/api/client');
    const client = new ApiClient('/api/v1');
    client.setAccessToken(token);
    const responses = await Promise.all(Array.from({ length: 6 }, () => client.request<{ products: unknown[] }>('/products')));
    let denial = 0;
    try { await client.request('/products', { method: 'POST', body: {} }); }
    catch (error) { denial = (error as { status: number }).status; }
    return { reads: responses.every(response => Array.isArray(response.products)), denial };
  }, expired);
  expect(result).toEqual({ reads: true, denial: 403 });
  expect(refreshes).toBe(1);
  await page.evaluate(async () => {
    const path = '/src/app/query-client.ts';
    const { queryClient } = await import(path) as typeof import('../src/app/query-client');
    queryClient.setQueryData(['private-test-cache'], { private: true });
  });
  const logout = await context.request.post('/api/v1/auth/logout', { headers: { Authorization: `Bearer ${data.access_token}` } });
  expect(logout.status()).toBe(204);
  const revoked = await page.evaluate(async () => {
    const path = '/src/lib/api/client.ts';
    const { api } = await import(path) as typeof import('../src/lib/api/client');
    try { await api.request('/products'); return 200; }
    catch (error) { return (error as { status: number }).status; }
  });
  expect(revoked).toBe(401);
  await expect(page).toHaveURL(/\/login$/);
  expect(await page.evaluate(async () => {
    const path = '/src/app/query-client.ts';
    const { queryClient } = await import(path) as typeof import('../src/app/query-client');
    return queryClient.getQueryCache().getAll().length;
  })).toBe(0);
});

test('unavailable network hides the workspace and retries the real cookie session after recovery', async ({ page, context }) => {
  await login(page, 'OPERATOR');
  // Import before going offline; this is the actual module used by the rendered app.
  await page.evaluate(async () => { const path = '/src/lib/api/client.ts'; await import(path); });
  await context.setOffline(true);
  await page.evaluate(async () => {
    const path = '/src/lib/api/client.ts';
    const { api } = await import(path) as typeof import('../src/lib/api/client');
    await api.refresh().catch(() => {});
  });
  await expect(page.getByRole('heading', { name: 'We couldn’t restore your session' })).toBeVisible();
  await expect(page.getByRole('navigation', { name: 'Main navigation' })).toHaveCount(0);
  await context.setOffline(false);
  await page.getByRole('button', { name: 'Try again' }).click();
  await expect(page.getByRole('heading', { name: 'Your warehouse workspace.' })).toBeVisible();
});
