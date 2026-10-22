import { describe, expect, it, vi } from 'vitest';
import { ApiClient } from './client';
import { ApiError } from './types';

const ok = (data: unknown) => Response.json({ success: true, data });
const failure = (status = 401) => Response.json({ success: false, error: { code: status === 503 ? 'unavailable' : 'unauthorized', message: 'Public error message' } }, { status });

describe('typed authentication client', () => {
  it('shares one cookie refresh across concurrent 401s, with one retry per request', async () => {
    let release!: () => void;
    const gate = new Promise<void>(resolve => { release = resolve; });
    let refreshes = 0;
    const fetcher = vi.fn<typeof fetch>(async (url, options) => {
      expect(options?.credentials).toBe('include');
      if (String(url).endsWith('/auth/refresh')) {
        refreshes++;
        await gate;
        return ok({ access_token: 'new-token', expires_in: 900 });
      }
      return new Headers(options?.headers).get('Authorization') === 'Bearer new-token' ? ok({ allowed: true }) : failure();
    });
    const client = new ApiClient('/api/v1', fetcher);
    client.setAccessToken('expired-token');
    const requests = Array.from({ length: 8 }, () => client.request('/products'));
    await vi.waitFor(() => expect(refreshes).toBe(1));
    release();
    expect(await Promise.all(requests)).toEqual(Array.from({ length: 8 }, () => ({ allowed: true })));
    expect(refreshes).toBe(1);
    expect(fetcher).toHaveBeenCalledTimes(17);
  });

  it('never retries a protected request more than once and clears a revoked session', async () => {
    const fetcher = vi.fn<typeof fetch>(async url => String(url).endsWith('/auth/refresh') ? ok({ access_token: 'new-token', expires_in: 900 }) : failure());
    const client = new ApiClient('/api/v1', fetcher);
    client.setAccessToken('old-token');
    await expect(client.request('/products')).rejects.toMatchObject({ status: 401, code: 'unauthorized' });
    expect(fetcher).toHaveBeenCalledTimes(3);
    expect(client.getAccessToken()).toBeNull();
  });

  it('clears expired/unavailable sessions while preserving public status/code/message', async () => {
    for (const status of [401, 409, 503]) {
      const client = new ApiClient('/api/v1', vi.fn<typeof fetch>(async () => failure(status)));
      const listener = vi.fn();
      client.subscribe(listener);
      client.setAccessToken('old-token');
      await expect(client.refresh()).rejects.toMatchObject({ status, message: 'Public error message' });
      expect(client.getAccessToken()).toBeNull();
      expect(listener).toHaveBeenLastCalledWith(null, expect.any(ApiError));
    }
  });

  it('does not restore a session cleared while refresh was in flight', async () => {
    let release!: () => void;
    const gate = new Promise<void>(resolve => { release = resolve; });
    const client = new ApiClient('/api/v1', vi.fn<typeof fetch>(async () => { await gate; return ok({ access_token: 'late-token', expires_in: 900 }); }));
    const pending = client.refresh();
    client.clearSession();
    release();
    await expect(pending).rejects.toMatchObject({ status: 401 });
    expect(client.getAccessToken()).toBeNull();
  });

  it('handles empty logout 204, skips public-auth retry and sends strict JSON payloads', async () => {
    const fetcher = vi.fn<typeof fetch>(async (url, options) => {
      if (String(url).endsWith('/auth/login')) {
        expect(options?.body).toBe(JSON.stringify({ email: 'operator@example.test', password: 'example-input', remember_me: false }));
        expect(new Headers(options?.headers).has('Authorization')).toBe(false);
        return failure();
      }
      return new Response(null, { status: 204 });
    });
    const client = new ApiClient('/api/v1', fetcher);
    await expect(client.request('/auth/logout', { method: 'POST' })).resolves.toBeUndefined();
    await expect(client.request('/auth/login', { method: 'POST', public: true, body: { email: 'operator@example.test', password: 'example-input', remember_me: false } })).rejects.toMatchObject({ status: 401 });
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it('distinguishes network failures and rejects malformed success envelopes', async () => {
    const offline = new ApiClient('/api/v1', vi.fn<typeof fetch>(async () => { throw new TypeError('network'); }));
    await expect(offline.refresh()).rejects.toMatchObject({ status: 0, code: 'network' });
    for (const data of [{ data: {} }, { success: true }, null]) {
      const client = new ApiClient('/api/v1', vi.fn<typeof fetch>(async () => Response.json(data)));
      await expect(client.request('/auth/login', { public: true })).rejects.toMatchObject({ code: 'invalid_response' });
    }
  });
});
