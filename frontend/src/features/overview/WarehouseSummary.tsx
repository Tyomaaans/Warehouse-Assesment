import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { queryKeys } from '../../app/query-client';
import { QueryError } from '../../components/ui/ListControls';
import { EmptyState, LoadingState } from '../../components/ui/States';
import { displayDate } from '../../lib/view';
import { useAuth } from '../auth/use-auth';
import { receivingApi } from '../receiving/api';
import { inventoryApi } from '../inventory/api';
import { notificationsApi, notificationPolling } from '../notifications/api';

export function WarehouseSummary() {
  const { user } = useAuth();
  const actor = user!.id;
  const incomplete = useQuery({ queryKey: [...queryKeys.receivings(actor), { page: 1, limit: 1, status: 'INCOMPLETE' }], queryFn: ({ signal }) => receivingApi.list({ page: 1, limit: 1, status: 'INCOMPLETE' }, signal) });
  const complete = useQuery({ queryKey: [...queryKeys.receivings(actor), { page: 1, limit: 1, status: 'COMPLETED' }], queryFn: ({ signal }) => receivingApi.list({ page: 1, limit: 1, status: 'COMPLETED' }, signal) });
  const inventory = useQuery({ queryKey: [...queryKeys.inventory(actor), 'balances', 1, 5, ''], queryFn: ({ signal }) => inventoryApi.balances(1, 5, '', signal) });
  const notifications = useQuery({ queryKey: [...queryKeys.notifications(actor), 1, 5], queryFn: ({ signal }) => notificationsApi.list(1, 5, signal), ...notificationPolling });
  const cards = [
    { label: 'Incomplete receivings', query: incomplete, path: '/receivings?status=INCOMPLETE' },
    { label: 'Completed receivings', query: complete, path: '/receivings?status=COMPLETED' },
    { label: 'SKUs with posted inventory', query: inventory, path: '/inventory' },
    { label: 'Your delivered notifications', query: notifications, path: '/notifications' },
  ];
  return <section aria-label="Warehouse summary"><div className="summary-grid">{cards.map(({ label, query, path }) => <article className="summary-card" key={label}><h2><Link className="text-link" to={path}>{label}</Link></h2>{query.isPending ? <LoadingState label={`Loading ${label.toLowerCase()}…`} /> : query.isError ? <QueryError error={query.error} retry={() => { void query.refetch(); }} /> : <strong className="summary-value">{query.data.meta.total_data.toLocaleString()}</strong>}</article>)}</div>
    <div className="summary-previews"><section className="data-card"><div className="card-heading"><h2>Inventory preview</h2><p>First five SKUs in catalogue order. Quantities are approved inbound balances.</p></div>{inventory.isSuccess && (inventory.data.balances.length ? <ul className="summary-list">{inventory.data.balances.map(row => <li key={row.id}><Link className="text-link" to={`/inventory?view=history&product_id=${row.product_id}`}>{row.sku}</Link><span>{row.quantity.toLocaleString()}</span></li>)}</ul> : <EmptyState title="No inventory posted">Approved receipts will appear here.</EmptyState>)}{!inventory.isSuccess && <p className="preview-note">See the inventory summary above for availability.</p>}</section>
      <section className="data-card"><div className="card-heading"><h2>Recent notifications</h2><p>Latest five updates for you. Read state applies to these rows only.</p></div>{notifications.isSuccess && (notifications.data.notifications.length ? <ul className="summary-list">{notifications.data.notifications.map(item => <li key={item.id}><div><Link className="text-link" to="/notifications">{item.message}</Link><small>{displayDate(item.created_at)} · {item.read_at ? 'Read' : 'Unread'}</small></div></li>)}</ul> : <EmptyState title="No delivered updates">Your receiving notifications will appear here.</EmptyState>)}{!notifications.isSuccess && <p className="preview-note">See the notification summary above for availability.</p>}</section></div>
  </section>;
}
