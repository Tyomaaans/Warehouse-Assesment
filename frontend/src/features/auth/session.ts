import { ApiError, type Role } from '../../lib/api/types';

export interface SessionUser { id: string; role: Role; expiresAt: number; email?: string }

// Claims are used for display only. Every backend request still verifies JWT/session/current role.
export function userFromToken(token: string): SessionUser {
  try {
    const part = token.split('.')[1];
    if (!part) throw new Error();
    const claims = JSON.parse(atob(part.replace(/-/g, '+').replace(/_/g, '/'))) as { sub?: string; role?: string; exp?: number };
    if (!claims.sub || !/^[\da-f]{8}-[\da-f]{4}-4[\da-f]{3}-[89ab][\da-f]{3}-[\da-f]{12}$/i.test(claims.sub) ||
        (claims.role !== 'OPERATOR' && claims.role !== 'SUPERVISOR') || typeof claims.exp !== 'number' || claims.exp * 1000 <= Date.now()) throw new Error();
    const bytes = claims.sub.replace(/-/g, '').match(/../g)!.map(hex => String.fromCharCode(parseInt(hex, 16))).join('');
    const id = btoa(bytes).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    return { id, role: claims.role, expiresAt: claims.exp * 1000 };
  } catch { throw new ApiError(502, 'invalid_response', 'The server returned an unexpected session. Please try signing in again.'); }
}

export const roleLabel = (role: Role) => role === 'SUPERVISOR' ? 'Supervisor' : 'Operator';
