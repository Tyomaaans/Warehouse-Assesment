import { useQuery } from '@tanstack/react-query';
import { Link, useLocation, useParams } from 'react-router-dom';
import { queryKeys } from '../../app/query-client';
import { PageHeader } from '../../components/ui/PageHeader';
import { Table } from '../../components/ui/Table';
import { EmptyState, ErrorState, LoadingState } from '../../components/ui/States';
import { QueryError } from '../../components/ui/ListControls';
import { displayDate } from '../../lib/view';
import { useAuth } from '../auth/use-auth';
import { isCanonicalID, receivingApi } from './api';
import { ReceivingStatus } from './ReceivingStatus';
import { TransactionHistory } from '../transactions/TransactionHistory';

export function ReceivingDetailPage() {
  const { id = '' } = useParams();
  const { user } = useAuth();
  const location = useLocation();
  const valid = isCanonicalID(id);
  const query = useQuery({ queryKey: [...queryKeys.receivings(user!.id), 'detail', id], queryFn: ({ signal }) => receivingApi.detail(id, signal), enabled: valid });
  if (!valid) return <ErrorState title="Invalid document identifier" action={<Link className="text-link" to="/receivings">Back to receiving documents</Link>}>Open a receiving document from the list using its canonical identifier.</ErrorState>;
  if (query.isPending) return <LoadingState label="Loading receiving document…" />;
  if (query.isError) return <><Link className="back-link" to="/receivings">← Receiving documents</Link><QueryError error={query.error} retry={() => { void query.refetch(); }} /></>;
  const document = query.data;
  return <><Link className="back-link" to="/receivings">← Receiving documents</Link><PageHeader eyebrow="RECEIVING DOCUMENT" title={document.order_reference} description="Expected quantities are saved with this document and cannot be edited." action={<ReceivingStatus status={document.status} />} />
    {(location.state as { created?: boolean } | null)?.created && <p className="success-notice" role="status">Receiving document created.</p>}
    <dl className="document-facts"><div><dt>Document ID</dt><dd className="identifier">{document.id}</dd></div><div><dt>Created</dt><dd>{displayDate(document.created_at)}</dd></div><div><dt>Created by</dt><dd className="identifier">{document.created_by}</dd></div><div><dt>Last updated</dt><dd>{displayDate(document.updated_at)}</dd></div>{document.completed_at && <div><dt>Completed</dt><dd>{displayDate(document.completed_at)}</dd></div>}</dl>
    <section className="data-card"><div className="card-heading"><h2>Expected SKU quantities</h2><p>Approved totals and remaining quantities come from the server. Only approved receiving transactions count toward completion.</p></div>
      {document.items.length ? <Table caption="Expected and approved SKU quantities" headings={['SKU', 'Product', 'Expected', 'Approved', 'Remaining']}>
        {document.items.map(item => <tr key={item.id}><td><span className="sku-label">{item.sku}</span></td><td>{item.name}</td><td className="quantity">{item.expected_quantity.toLocaleString()}</td><td className="quantity">{item.approved_quantity.toLocaleString()}</td><td className="quantity">{item.remaining_quantity.toLocaleString()}</td></tr>)}
      </Table> : <EmptyState title="No SKU lines returned">The server returned no expected lines for this document.</EmptyState>}
    </section>
    <TransactionHistory key={document.id} receiving={document} />
  </>;
}
