import { useQuery } from '@tanstack/react-query';
import { Link, useSearchParams } from 'react-router-dom';
import { queryKeys } from '../../app/query-client';
import { Table } from '../../components/ui/Table';
import { EmptyState, LoadingState } from '../../components/ui/States';
import { Pagination, PageSize, QueryError } from '../../components/ui/ListControls';
import { displayDate, pageOptions } from '../../lib/view';
import { useAuth } from '../auth/use-auth';
import type { ReceivingDetail } from '../receiving/api';
import { isActive, transactionsApi } from './api';
import { TransactionStatus } from './TransactionStatus';

export function TransactionHistory({ receiving }: { receiving: ReceivingDetail }) {
  const { user } = useAuth();
  const [params, setParams] = useSearchParams();
  const { page, limit } = pageOptions(params);
  const query = useQuery({ queryKey: [...queryKeys.transactions(user!.id, receiving.id), page, limit], queryFn: ({ signal }) => transactionsApi.list(receiving.id, page, limit, signal) });
  const active = query.data?.transactions.find(isActive);
  function change(nextPage: number, nextLimit = limit) { setParams({ page: String(nextPage), limit: String(nextLimit) }); }
  return <section className="data-card" aria-label="Transaction history"><div className="card-heading history-heading"><div><h2>Transaction history</h2><p>Recorded physical receipts, including rejected attempts. Only approved transactions affect inventory.</p></div>{user?.role === 'OPERATOR' && receiving.status === 'INCOMPLETE' && !query.isPending && !query.isError && !active && <Link className="button button-primary" to={`/receivings/${receiving.id}/transactions/new`}>Record physical receipt</Link>}</div>
    {active && <p className="context-notice">An active transaction blocks another receipt. <Link className="text-link" to={`/transactions/${active.id}`}>Open active transaction</Link></p>}
    {receiving.status === 'COMPLETED' && <p className="context-notice">This receiving document is completed and closed to new transactions.</p>}
    {query.isPending ? <LoadingState label="Loading transaction history…" /> : query.isError ? <QueryError error={query.error} retry={() => { void query.refetch(); }} /> : <>
      <div className="history-toolbar"><PageSize value={limit} onChange={size => change(1, size)} /></div>
      {query.data.transactions.length ? <Table caption="Receiving transaction history" headings={['Transaction', 'Status / review', 'Submitted quantities', 'Recorded']}>
        {query.data.transactions.map(transaction => <tr key={transaction.id}><td><Link className="text-link identifier" to={`/transactions/${transaction.id}`}>{transaction.id}</Link></td><td><TransactionStatus transaction={transaction} /></td><td>{transaction.items.map(item => <div key={item.id} className="history-quantity">{receiving.items.find(line => line.id === item.receiving_document_item_id)?.sku ?? item.receiving_document_item_id}: <strong>{item.actual_quantity.toLocaleString()}</strong></div>)}</td><td><time dateTime={transaction.created_at}>{displayDate(transaction.created_at)}</time></td></tr>)}
      </Table> : <EmptyState title="No transactions on this page">{user?.role === 'OPERATOR' ? 'Record a physical receipt when goods arrive, or return to a previous history page.' : 'Operators record physical receipts. Choose another history page to inspect earlier transactions.'}</EmptyState>}
      <Pagination meta={query.data.meta} onPage={change} busy={query.isFetching} />
    </>}
  </section>;
}
