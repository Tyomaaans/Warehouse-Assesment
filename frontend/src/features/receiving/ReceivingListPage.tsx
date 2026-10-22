import { useQuery } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { Plus, ArrowUpRight } from 'lucide-react';
import { queryKeys } from '../../app/query-client';
import { PageHeader } from '../../components/ui/PageHeader';
import { Table } from '../../components/ui/Table';
import { EmptyState, LoadingState } from '../../components/ui/States';
import { PageSize, Pagination, QueryError } from '../../components/ui/ListControls';
import { displayDate, pageOptions } from '../../lib/view';
import { useAuth } from '../auth/use-auth';
import { receivingApi, type ReceivingFilters } from './api';
import { ReceivingStatus } from './ReceivingStatus';

export function ReceivingListPage({ mode = 'documents' }: { mode?: 'documents' | 'transactions' | 'review' }) {
  const { user } = useAuth();
  const [params, setParams] = useSearchParams();
  const status = params.get('status');
  const filters: ReceivingFilters = { ...pageOptions(params), status: status === 'INCOMPLETE' || status === 'COMPLETED' ? status : '' };
  const query = useQuery({ queryKey: [...queryKeys.receivings(user!.id), 'list', filters], queryFn: ({ signal }) => receivingApi.list(filters, signal) });
  function change(page: number, nextStatus = filters.status, limit = filters.limit) {
    const next = new URLSearchParams({ page: String(page), limit: String(limit) });
    if (nextStatus) next.set('status', nextStatus);
    setParams(next);
  }
  const title = mode === 'review' ? 'Supervisor review' : mode === 'transactions' ? 'Receiving transactions' : 'Receiving documents';
  return <><PageHeader eyebrow="INBOUND EXPECTATIONS" title={title} description={mode === 'documents' ? 'Expected quantities and approved progress for every inbound delivery.' : 'Choose a receiving document to open its transaction history and permitted actions.'} action={mode === 'documents' && user?.role === 'OPERATOR' ? <Link className="button button-primary" to="/receivings/new"><Plus size={16} aria-hidden="true" />New receiving document</Link> : undefined} />
    <section className="data-card" aria-label="Receiving documents"><div className="list-toolbar"><label className="inline-field">Status<select value={filters.status} onChange={event => change(1, event.target.value as ReceivingFilters['status'])}><option value="">All statuses</option><option value="INCOMPLETE">Incomplete</option><option value="COMPLETED">Completed</option></select></label><PageSize value={filters.limit} onChange={limit => change(1, filters.status, limit)} /></div>
      {query.isPending ? <LoadingState label="Loading receiving documents…" /> : query.isError ? <QueryError error={query.error} retry={() => { void query.refetch(); }} /> : <>
        {query.data.receivings.length ? <Table caption="Receiving documents" headings={['External ORD reference', 'Status', 'Created', 'Document']}>
          {query.data.receivings.map(document => <tr key={document.id}><td><Link className="document-link" to={`/receivings/${document.id}`}>{document.order_reference}<ArrowUpRight size={14} aria-hidden="true" /></Link></td><td><ReceivingStatus status={document.status} /></td><td><time dateTime={document.created_at}>{displayDate(document.created_at)}</time></td><td><span className="identifier">{document.id}</span></td></tr>)}
        </Table> : <EmptyState title="No receiving documents on this page">{filters.status ? 'Try another status or return to a previous page.' : user?.role === 'OPERATOR' ? 'Create a document to record expected SKU quantities, or return to a previous page.' : 'Operators create receiving documents. You can also return to a previous page.'}</EmptyState>}
        <Pagination meta={query.data.meta} onPage={page => change(page)} busy={query.isFetching} />
      </>}
    </section>
  </>;
}
