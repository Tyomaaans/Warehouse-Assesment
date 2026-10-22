import { api } from '../../lib/api/client';
import type { PaginationMeta } from '../../lib/api/types';

export interface Product { id: string; sku: string; name: string; created_at: string }
export interface ProductsPage { products: Product[]; meta: PaginationMeta }
export interface ProductInput { sku: string; name: string }
export interface ProductFilters { page: number; limit: number; search: string }

export const productsApi = {
  list: (filters: ProductFilters, signal?: AbortSignal) => {
    const query = new URLSearchParams({ page: String(filters.page), limit: String(filters.limit) });
    if (filters.search) query.set('search', filters.search);
    return api.request<ProductsPage>(`/products?${query}`, { signal });
  },
  create: (body: ProductInput) => api.request<Product>('/products', { method: 'POST', body }),
};
