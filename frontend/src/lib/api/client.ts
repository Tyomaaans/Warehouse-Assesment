import { API_BASE } from './config';
import { ApiError, type Failure, type RefreshData, type Success } from './types';

interface RequestOptions extends Omit<RequestInit, 'body' | 'credentials'> {
  body?: unknown;
  public?: boolean;
}
type SessionListener = (token: string | null, error?: ApiError) => void;

// Adapted from the reference typed fetch client. Tokens exist only in this instance's memory.
export class ApiClient {
  private token: string | null = null;
  private refreshPromise: Promise<string> | null = null;
  private revision = 0;
  private listeners = new Set<SessionListener>();

  constructor(private base = API_BASE, private fetcher: typeof fetch = (...args) => fetch(...args)) {}

  getAccessToken() { return this.token; }
  subscribe(listener: SessionListener) {
    this.listeners.add(listener);
    return () => { this.listeners.delete(listener); };
  }
  setAccessToken(token: string) {
    this.token = token;
    for (const listener of this.listeners) listener(token);
  }
  clearSession(error?: ApiError) {
    this.revision++;
    this.token = null;
    for (const listener of this.listeners) listener(null, error);
  }

  refresh(): Promise<string> {
    if (this.refreshPromise) return this.refreshPromise;
    const revision = this.revision;
    this.refreshPromise = (async () => {
      try {
        const data = await this.request<RefreshData>('/auth/refresh', { method: 'POST', public: true });
        if (revision !== this.revision) throw new ApiError(401, 'unauthorized', 'This session has ended. Please sign in again.');
        if (!data.access_token || data.expires_in !== 900) throw new ApiError(502, 'invalid_response', 'The server returned an unexpected session response.');
        this.setAccessToken(data.access_token);
        return data.access_token;
      } catch (error) {
        if (revision === this.revision) this.clearSession(error instanceof ApiError ? error : new ApiError(0, 'network', 'Unable to reach the server.'));
        throw error;
      } finally { this.refreshPromise = null; }
    })();
    return this.refreshPromise;
  }

  async request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const { body, public: isPublic, headers, ...rest } = options;
    const send = async () => {
      const finalHeaders = new Headers(headers);
      finalHeaders.set('Accept', 'application/json');
      if (body !== undefined) finalHeaders.set('Content-Type', 'application/json');
      if (!isPublic && this.token) finalHeaders.set('Authorization', `Bearer ${this.token}`);
      try {
        return await this.fetcher(`${this.base}${path}`, { ...rest, headers: finalHeaders, credentials: 'include',
          body: body === undefined ? undefined : JSON.stringify(body) });
      } catch (error) {
        if (error instanceof Error && error.name === 'AbortError') throw error;
        throw new ApiError(0, 'network', 'Unable to reach the server. Check your connection and try again.');
      }
    };
    const usedToken = this.token;
    let response = await send();
    if (response.status === 401 && !isPublic) {
      // A delayed request using the previous token can share an already-completed rotation too.
      if (!this.token || this.token === usedToken) await this.refresh();
      response = await send(); // Exactly one retry, including when the retry is another 401.
    }
    if (!response.ok) {
      let detail: Failure | null = null;
      try { detail = await response.json() as Failure; } catch { /* Proxy failures may not have JSON. */ }
      const error = new ApiError(response.status, detail?.error?.code ?? 'request_failed',
        detail?.error?.message ?? 'The request could not be completed. Please try again.');
      if (response.status === 401 && !isPublic) this.clearSession(error);
      throw error;
    }
    if (response.status === 204) return undefined as T;
    let envelope: Success<T>;
    try { envelope = await response.json() as Success<T>; }
    catch { throw new ApiError(502, 'invalid_response', 'The server returned an unexpected response.'); }
    if (envelope?.success !== true || !('data' in envelope)) throw new ApiError(502, 'invalid_response', 'The server returned an unexpected response.');
    return envelope.data;
  }
}

export const api = new ApiClient();
