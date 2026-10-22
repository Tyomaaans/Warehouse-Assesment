import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { AuthContext, type AuthContextValue } from './context';
import { LoginPage } from './LoginPage';
import { ProtectedRoute } from './ProtectedRoute';
import { userFromToken } from './session';
import { ApiError } from '../../lib/api/types';

afterEach(cleanup);
function auth(overrides: Partial<AuthContextValue> = {}): AuthContextValue {
  return { status: 'anonymous', user: null, signingOut: false, logoutNotice: null, login: vi.fn(), logout: vi.fn(), restore: vi.fn(), ...overrides };
}
function loginView(value: AuthContextValue) {
  render(<AuthContext.Provider value={value}><MemoryRouter><LoginPage /></MemoryRouter></AuthContext.Provider>);
}

describe('login feedback and role protection', () => {
  it('validates labeled inputs and focuses the first invalid field without submitting', async () => {
    const value = auth();
    loginView(value);
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));
    expect(screen.getByText('Enter your email address.')).toBeTruthy();
    expect(screen.getByText('Enter your password.')).toBeTruthy();
    expect(document.activeElement).toBe(screen.getByLabelText('Email address'));
    expect(value.login).not.toHaveBeenCalled();
  });

  it('submits only the locked login payload and displays nested server errors', async () => {
    const value = auth({ login: vi.fn().mockRejectedValue(new ApiError(401, 'invalid_credentials', 'Invalid email or password')) });
    loginView(value);
    await userEvent.type(screen.getByLabelText('Email address'), 'operator@example.test');
    await userEvent.type(screen.getByLabelText('Password', { exact: true }), 'example-input');
    await userEvent.click(screen.getByLabelText('Keep me signed in'));
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));
    expect(value.login).toHaveBeenCalledWith({ email: 'operator@example.test', password: 'example-input', remember_me: true });
    expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'Invalid email or password');
  });

  it('blocks Operator review access while allowing Supervisor access', () => {
    for (const role of ['OPERATOR', 'SUPERVISOR'] as const) {
      const value = auth({ status: 'authenticated', user: { id: 'VQ6EAOKbQdSnFkRmVUQAAA', role, expiresAt: Date.now() + 900_000 } });
      render(<AuthContext.Provider value={value}><MemoryRouter initialEntries={['/review']}><Routes><Route element={<ProtectedRoute roles={['SUPERVISOR']} />}><Route path="/review" element={<h1>Review placeholder</h1>} /></Route></Routes></MemoryRouter></AuthContext.Provider>);
      expect(screen.getByRole('heading', { name: role === 'SUPERVISOR' ? 'Review placeholder' : 'Supervisor access required' })).toBeTruthy();
      cleanup();
    }
  });

  it('shows retryable unavailable state without revealing protected content', async () => {
    const value = auth({ status: 'unavailable', message: 'Service temporarily unavailable' });
    loginView(value);
    expect(screen.getByRole('alert').textContent).toContain('Service temporarily unavailable');
    expect(screen.queryByRole('form')).toBeNull();
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }));
    expect(value.restore).toHaveBeenCalledOnce();
  });

  it('derives canonical ID/role for display, and rejects expired or unsupported claims', () => {
    const token = (role: string, exp: number) => `header.${btoa(JSON.stringify({ sub: '550e8400-e29b-41d4-a716-446655440000', role, exp }))}.signature`;
    expect(userFromToken(token('OPERATOR', Math.floor(Date.now() / 1000) + 900))).toMatchObject({ id: 'VQ6EAOKbQdSnFkRmVUQAAA', role: 'OPERATOR' });
    expect(() => userFromToken(token('ADMIN', Date.now()))).toThrow(ApiError);
    expect(() => userFromToken(token('OPERATOR', 1))).toThrow(ApiError);
  });
});
