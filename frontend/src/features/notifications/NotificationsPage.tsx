import { useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { queryKeys } from '../../app/query-client';
import { Button } from '../../components/ui/Button';
import { PageHeader } from '../../components/ui/PageHeader';
import { EmptyState, LoadingState } from '../../components/ui/States';
import { Pagination, PageSize, QueryError } from '../../components/ui/ListControls';
import { displayDate, pageOptions } from '../../lib/view';
import { useAuth } from '../auth/use-auth';
import { notificationsApi, notificationPolling } from './api';

export function NotificationsPage() {
  const { user } = useAuth();
  const cache = useQueryClient();
  const [params, setParams] = useSearchParams();
  const { page, limit } = pageOptions(params);
  const [error, setError] = useState('');
  const lock = useRef(false);
  const query = useQuery({ queryKey: [...queryKeys.notifications(user!.id), page, limit], queryFn: ({ signal }) => notificationsApi.list(page, limit, signal), ...notificationPolling });
  const mutation = useMutation({ mutationFn: (id: string) => notificationsApi.read(id), onSettled: async () => { await cache.invalidateQueries({ queryKey: queryKeys.notifications(user!.id) }); } });
  async function read(id: string) {
    if (lock.current) return;
    lock.current = true; setError('');
    try { await mutation.mutateAsync(id); } catch (failure) { setError((failure as Error).message); } finally { lock.current = false; }
  }
  function change(next: number, size = limit) { setParams({ page: String(next), limit: String(size) }); }
  return <><PageHeader eyebrow="YOUR RECEIVING UPDATES" title="Notifications" description="Updates addressed to your account. Refreshes every minute while this view is visible." action={<Button variant="secondary" disabled={query.isFetching} onClick={() => { void query.refetch(); }}>Refresh notifications</Button>} />
    {error && <p role="alert" className="form-alert">{error}</p>}
    <section className="data-card" aria-label="Your notifications"><div className="list-toolbar"><p>Read and unread updates</p><PageSize value={limit} onChange={size => change(1, size)} /></div>
      {query.isPending ? <LoadingState label="Loading notifications…" /> : query.isError ? <QueryError error={query.error} retry={() => { void query.refetch(); }} /> : <>
        {query.data.notifications.length ? <ul className="notification-list">{query.data.notifications.map(item => <li key={item.id} className={item.read_at ? '' : 'unread'}><div className="notification-copy"><span className="pill">{item.read_at ? 'Read' : 'Unread'}</span><p>{item.message}</p><time dateTime={item.created_at}>{displayDate(item.created_at)}</time>{item.read_at && <small>Read {displayDate(item.read_at)}</small>}{item.receiving_transaction_id && <Link className="text-link" to={`/transactions/${item.receiving_transaction_id}`}>Open transaction</Link>}</div>{!item.read_at && <Button variant="secondary" disabled={mutation.isPending} onClick={() => { void read(item.id); }} aria-label={`Mark as read: ${item.message}`}>{mutation.isPending && mutation.variables === item.id ? 'Saving…' : 'Mark as read'}</Button>}</li>)}</ul> : <EmptyState title="No notifications on this page">Receiving updates will appear here when delivered. You can also return to a previous page.</EmptyState>}
        <Pagination meta={query.data.meta} onPage={change} busy={query.isFetching} />
      </>}
    </section>
  </>;
}
