import { useState, type FormEvent } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { Plus, Search } from 'lucide-react';
import { queryKeys } from '../../app/query-client';
import { Button } from '../../components/ui/Button';
import { PageHeader } from '../../components/ui/PageHeader';
import { Table } from '../../components/ui/Table';
import { EmptyState, LoadingState } from '../../components/ui/States';
import { Pagination, PageSize, QueryError } from '../../components/ui/ListControls';
import { displayDate, pageOptions } from '../../lib/view';
import { useAuth } from '../auth/use-auth';
import { productsApi } from './api';
import { ProductForm } from './ProductForm';

export function ProductsPage() {
  const { user } = useAuth();
  const [params, setParams] = useSearchParams();
  const filters = { ...pageOptions(params), search: (params.get('search') ?? '').slice(0, 100) };
  const [creating, setCreating] = useState(false);
  const [notice, setNotice] = useState('');
  const query = useQuery({ queryKey: [...queryKeys.products(user!.id), filters], queryFn: ({ signal }) => productsApi.list(filters, signal) });
  function search(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setParams({ page: '1', limit: String(filters.limit), search: String(new FormData(event.currentTarget).get('search') ?? '') });
  }
  function change(page: number, limit = filters.limit) { setParams({ page: String(page), limit: String(limit), search: filters.search }); }

  return <><PageHeader eyebrow="PRODUCT CATALOGUE" title="Products" description="A shared catalogue of your warehouse SKUs." action={user?.role === 'SUPERVISOR' && !creating ? <Button onClick={() => { setCreating(true); setNotice(''); }}><Plus size={16} aria-hidden="true" />New product</Button> : undefined} />
    {notice && <p className="success-notice" role="status">{notice}</p>}
    {creating && <ProductForm onCancel={() => setCreating(false)} onCreated={product => { setCreating(false); setNotice(`Product ${product.sku} created.`); setParams({ page: '1', limit: String(filters.limit) }); }} />}
    <section className="data-card" aria-label="Product catalogue"><div className="list-toolbar"><form onSubmit={search} className="search-form" role="search" key={filters.search}><label className="sr-only" htmlFor="product-search">Search products</label><Search size={17} aria-hidden="true" /><input id="product-search" name="search" type="search" maxLength={100} defaultValue={filters.search} placeholder="Search SKU or product name" /><Button type="submit" variant="secondary">Search</Button></form><PageSize value={filters.limit} onChange={limit => change(1, limit)} /></div>
      {query.isPending ? <LoadingState label="Loading products…" /> : query.isError ? <QueryError error={query.error} retry={() => { void query.refetch(); }} /> : <>
        {query.data.products.length ? <Table caption="Products" headings={['SKU', 'Product name', 'Created']}>
          {query.data.products.map(product => <tr key={product.id}><td><span className="sku-label">{product.sku}</span></td><td>{product.name}</td><td><time dateTime={product.created_at}>{displayDate(product.created_at)}</time></td></tr>)}
        </Table> : <EmptyState title={filters.search ? 'No matching products' : 'No products on this page'}>{filters.search ? 'Try another SKU or product name.' : user?.role === 'SUPERVISOR' ? 'Create a product to start building the catalogue, or return to a previous page.' : 'A Supervisor can add products to the catalogue. You can also return to a previous page.'}</EmptyState>}
        <Pagination meta={query.data.meta} onPage={change} busy={query.isFetching} />
      </>}
    </section>
  </>;
}
