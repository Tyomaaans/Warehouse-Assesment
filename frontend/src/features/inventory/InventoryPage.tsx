import type { FormEvent } from 'react';
import { useQueries, useQuery } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { queryKeys } from '../../app/query-client';
import { Button } from '../../components/ui/Button';
import { PageHeader } from '../../components/ui/PageHeader';
import { Table } from '../../components/ui/Table';
import { EmptyState, ErrorState, LoadingState } from '../../components/ui/States';
import { Pagination, PageSize, QueryError } from '../../components/ui/ListControls';
import { displayDate, pageOptions } from '../../lib/view';
import { useAuth } from '../auth/use-auth';
import { isCanonicalID, receivingApi } from '../receiving/api';
import { inventoryApi } from './api';

export function InventoryPage() {
  const { user } = useAuth();
  const [params, setParams] = useSearchParams();
  const ledger = params.get('view') === 'history';
  const { page, limit } = pageOptions(params);
  const search = (params.get('search') ?? '').slice(0, 100);
  const product = params.get('product_id') ?? '';
  const valid = !product || isCanonicalID(product);
  const balances = useQuery({ queryKey: [...queryKeys.inventory(user!.id), 'balances', page, limit, search], queryFn: ({ signal }) => inventoryApi.balances(page, limit, search, signal), enabled: !ledger });
  const movements = useQuery({ queryKey: [...queryKeys.inventory(user!.id), 'movements', page, limit, product], queryFn: ({ signal }) => inventoryApi.movements(page, limit, product, signal), enabled: ledger && valid });
  // Ledger DTOs expose product IDs. Join only this page's source documents for real SKU labels.
  const sourceIDs = [...new Set(movements.data?.movements.map(row => row.receiving_document_id) ?? [])];
  const sources = useQueries({ queries: sourceIDs.map(id => ({ queryKey: [...queryKeys.receivings(user!.id), 'detail', id], queryFn: ({ signal }: { signal: AbortSignal }) => receivingApi.detail(id, signal), enabled: ledger && valid })) });
  const products = new Map(sources.flatMap(source => source.data?.items.map(item => [item.product_id, item] as const) ?? []));
  const query = ledger ? movements : balances;
  function change(next: number, size = limit) { const nextParams = new URLSearchParams(params); nextParams.set('page', String(next)); nextParams.set('limit', String(size)); setParams(nextParams); }
  function find(event: FormEvent<HTMLFormElement>) { event.preventDefault(); setParams({ search: String(new FormData(event.currentTarget).get('search') ?? '').trim(), page: '1', limit: String(limit) }); }
  return <><PageHeader eyebrow="APPROVED INBOUND" title="Inventory" description="Balances and immutable movements from approved receipts. Pending and rejected receipts do not post stock." action={<Button variant="secondary" onClick={() => { void query.refetch(); }}>Refresh inventory</Button>} />
    <nav className="view-tabs" aria-label="Inventory views"><Link aria-current={!ledger ? 'page' : undefined} to="/inventory">Balances</Link><Link aria-current={ledger ? 'page' : undefined} to="/inventory?view=history">Inbound history</Link></nav>
    {!valid ? <ErrorState title="Invalid product identifier">Choose a product from inventory balances.</ErrorState> : <section className="data-card" aria-label={ledger ? 'Inbound history' : 'Inventory balances'}><div className="list-toolbar">{ledger ? <p>{product ? <>Product: <span className="identifier">{product}</span> · <Link to="/inventory?view=history" className="text-link">Clear filter</Link></> : 'All approved inbound movements'}</p> : <form key={search} className="search-form" role="search" onSubmit={find}><label className="sr-only" htmlFor="inventory-search">Search inventory</label><input id="inventory-search" type="search" name="search" defaultValue={search} maxLength={100} placeholder="Search SKU or product name" /><Button type="submit" variant="secondary">Search</Button></form>}<PageSize value={limit} onChange={size => change(1, size)} /></div>
      {query.isPending ? <LoadingState label="Loading inventory…" /> : query.isError ? <QueryError error={query.error} retry={() => { void query.refetch(); }} /> : ledger ? <>
        {sources.some(source => source.isError) && <p role="status" className="context-notice">Some SKU labels are unavailable. Source product identifiers and receiving links remain available.</p>}
        {movements.data?.movements.length ? <Table caption="Approved inbound ledger" headings={['SKU / product', 'Quantity received', 'Source transaction', 'Source receiving', 'Posted']}>
          {movements.data.movements.map(row => <tr key={row.id}><td><strong>{products.get(row.product_id)?.sku ?? 'Product'}</strong>{products.get(row.product_id)?.name && <p>{products.get(row.product_id)!.name}</p>}<div className="identifier">{row.product_id}</div></td><td className="quantity">+{row.quantity_delta.toLocaleString()}</td><td><Link className="text-link identifier" to={`/transactions/${row.receiving_transaction_id}`}>{row.receiving_transaction_id}</Link></td><td><Link className="text-link identifier" to={`/receivings/${row.receiving_document_id}`}>{row.receiving_document_id}</Link></td><td>{displayDate(row.created_at)}</td></tr>)}
        </Table> : <EmptyState title="No inbound movements on this page">Inventory history appears after Supervisor approval. Try another page or clear the product filter.</EmptyState>}
        {movements.data && <Pagination meta={movements.data.meta} onPage={change} busy={movements.isFetching} />}
      </> : <>
        {balances.data?.balances.length ? <Table caption="Approved inbound balances" headings={['SKU', 'Product', 'Approved balance', 'Updated', 'History']}>
          {balances.data.balances.map(row => <tr key={row.id}><td><strong>{row.sku}</strong></td><td>{row.name}</td><td className="quantity">{row.quantity.toLocaleString()}</td><td>{displayDate(row.updated_at)}</td><td><Link className="text-link" to={`/inventory?view=history&product_id=${row.product_id}`}>View history<span className="sr-only"> for {row.sku}</span></Link></td></tr>)}
        </Table> : <EmptyState title="No inventory on this page">Try another search or page. Products appear here after their first approved receipt.</EmptyState>}
        {balances.data && <Pagination meta={balances.data.meta} onPage={change} busy={balances.isFetching} />}
      </>}
    </section>}
  </>;
}
