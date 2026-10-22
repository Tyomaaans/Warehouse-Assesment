export type Role = 'OPERATOR' | 'SUPERVISOR';
export interface User { id: string; email: string; role: Role }
export interface LoginData { user: User; access_token: string; expires_in: 900 }
export interface RefreshData { access_token: string; expires_in: 900 }
export interface Success<T> { success: true; message?: string; data: T }
export interface Failure { success: false; error: { code: string; message: string } }
export interface PaginationMeta { current_page: number; total_pages: number; page_size: number; total_data: number }

export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}
