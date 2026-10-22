import { useRef, useState, type FormEvent } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useLocation, useParams } from 'react-router-dom';
import { queryKeys } from '../../app/query-client';
import { Button } from '../../components/ui/Button';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { TextField } from '../../components/ui/TextField';
import { PageHeader } from '../../components/ui/PageHeader';
import { Table } from '../../components/ui/Table';
import { ErrorState, LoadingState } from '../../components/ui/States';
import { QueryError } from '../../components/ui/ListControls';
import { displayDate, focusInvalid } from '../../lib/view';
import { useAuth } from '../auth/use-auth';
import { isCanonicalID, receivingApi } from '../receiving/api';
import { ReceivingStatus } from '../receiving/ReceivingStatus';
import { ActualQuantityForm } from './ActualQuantityForm';
import { isUnresolved, transactionsApi, type ActualItem } from './api';
import { refreshReceiving, transactionKey } from './queries';
import { TransactionStatus } from './TransactionStatus';

type Action = { type: 'approve' } | { type: 'review'; note: string } | { type: 'correct'; items: ActualItem[]; reason: string };

export function TransactionDetailPage() {
  const { id = '' } = useParams();
  const { user } = useAuth();
  const location = useLocation();
  const cache = useQueryClient();
  const lock = useRef(false);
  const [form, setForm] = useState<'correct' | 'review' | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [notice, setNotice] = useState('');
  const [error, setError] = useState('');
  const [note, setNote] = useState('');
  const [noteError, setNoteError] = useState('');
  const query = useQuery({ queryKey: transactionKey(user!.id, id), queryFn: ({ signal }) => transactionsApi.detail(id, signal), enabled: isCanonicalID(id) });
  const receivingID = query.data?.receiving_document_id ?? '';
  const document = useQuery({ queryKey: [...queryKeys.receivings(user!.id), 'detail', receivingID], queryFn: ({ signal }) => receivingApi.detail(receivingID, signal), enabled: Boolean(receivingID) });
  const mutation = useMutation({ mutationFn: (action: Action) => action.type === 'approve' ? transactionsApi.approve(id) : action.type === 'correct' ? transactionsApi.correct(id, action.items, action.reason) : transactionsApi.review(id, action.note), onSuccess: async (transaction, action) => {
    setConfirming(false); setForm(null);
    setNotice(action.type === 'approve' ? 'Transaction approved. Receiving progress is updated.' : action.type === 'review' ? 'Review resolved. The transaction remains rejected and inventory is unchanged. A new receipt may now be recorded.' : transaction.status === 'PENDING_APPROVAL' ? 'Correction saved on the same transaction. It is now pending approval; inventory is unchanged.' : 'Correction saved. It remains rejected and unresolved; inventory is unchanged.');
    await refreshReceiving(cache, user!.id, transaction.receiving_document_id, transaction);
  }, onError: async failure => {
    setConfirming(false); setError(failure.message);
    await Promise.all([cache.invalidateQueries({ queryKey: transactionKey(user!.id, id) }), refreshReceiving(cache, user!.id, receivingID)]);
  } });
  async function change(action: Action) {
    const current = query.data;
    if (lock.current || user?.role !== 'SUPERVISOR' || !current || (action.type === 'approve' ? current.status !== 'PENDING_APPROVAL' : !isUnresolved(current))) return;
    lock.current = true; setError(''); setNotice('');
    try { await mutation.mutateAsync(action); } catch { /* onError displays the server error and refreshes stale state. */ }
    finally { lock.current = false; }
  }
  if (!isCanonicalID(id)) return <ErrorState title="Invalid transaction identifier">Open a transaction from receiving history.</ErrorState>;
  if (query.isPending || (receivingID && document.isPending)) return <LoadingState label="Loading transaction…" />;
  if (query.isError || document.isError) return <QueryError error={(query.error ?? document.error)!} retry={() => { void query.refetch(); void document.refetch(); }} />;
  if (!document.data) return <LoadingState label="Loading receiving context…" />;
  const receiving = document.data;
  const transaction = query.data;
  const supervisor = user?.role === 'SUPERVISOR';
  const unresolved = isUnresolved(transaction);
  const recorded = (location.state as { recorded?: boolean } | null)?.recorded;
  function review(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!note.trim()) { setNoteError('Enter a verification note for the confirmed physical excess.'); focusInvalid(event.currentTarget); return; }
    void change({ type: 'review', note: note.trim() });
  }
  function actor(label: string, actorID: string | null, at: string | null) { return actorID && at ? <div><dt>{label}</dt><dd><span className="identifier">{actorID}</span><br />{displayDate(at)}</dd></div> : null; }
  return <><Link className="back-link" to={`/receivings/${receivingID}`}>← Receiving document and history</Link><PageHeader eyebrow="RECEIVING TRANSACTION" title="Transaction detail" description={transaction.id} action={<TransactionStatus transaction={transaction} />} />
    {error && <p className="form-alert" role="alert">{error} Refresh the current state before retrying.</p>}
    {notice && <p className="success-notice" role="status">{notice}</p>}
    {recorded && !notice && <p className={transaction.status === 'REJECTED_BY_SYSTEM' ? 'context-notice' : 'success-notice'} role="status">{transaction.status === 'REJECTED_BY_SYSTEM' ? `Physical receipt recorded as REJECTED_BY_SYSTEM / ${transaction.review_status}. The attempt is saved; inventory is unchanged.` : transaction.status === 'APPROVED' ? 'This recorded receipt is approved. Current receiving quantities come from the server.' : 'Physical receipt recorded. Awaiting Supervisor approval; inventory has not been posted by submission.'}</p>}
    <div className="transaction-document"><Link className="text-link" to={`/receivings/${receivingID}`}>{document.data.order_reference}</Link><ReceivingStatus status={document.data.status} /><Button variant="quiet" disabled={mutation.isPending} onClick={() => { void query.refetch(); void document.refetch(); }}>Refresh transaction</Button></div>
    <dl className="document-facts"><div><dt>Transaction ID</dt><dd className="identifier">{transaction.id}</dd></div><div><dt>Receiving document ID</dt><dd className="identifier">{receivingID}</dd></div>{actor('Recorded by', transaction.created_by, transaction.created_at)}<div><dt>Last updated</dt><dd>{displayDate(transaction.updated_at)}</dd></div>{actor('Approved by', transaction.approved_by, transaction.approved_at)}{actor('Latest correction by', transaction.corrected_by, transaction.corrected_at)}{actor('Review resolved by', transaction.reviewed_by, transaction.reviewed_at)}</dl>
    {transaction.rejection_reason && <p className="form-alert"><strong>System rejection reason:</strong> {transaction.rejection_reason}</p>}
    {transaction.review_note && <section className="snapshot-note"><h2>Supervisor verification note</h2><p>{transaction.review_note}</p></section>}
    <section className="data-card"><div className="card-heading"><h2>Submitted quantities</h2><p>Expected, approved and remaining quantities reflect the receiving document’s current progress.</p></div><Table caption="Transaction quantities" headings={['SKU / product', 'Submitted', 'Expected', 'Current approved total', 'Remaining']}>
      {transaction.items.map(item => {
        const line = receiving.items.find(entry => entry.id === item.receiving_document_item_id);
        return <tr key={item.id}><td>{line ? <><strong>{line.sku}</strong><br />{line.name}</> : <span className="identifier">{item.receiving_document_item_id}</span>}</td><td className="quantity">{item.actual_quantity.toLocaleString()}</td><td className="quantity">{line?.expected_quantity.toLocaleString() ?? 'Unavailable'}</td><td className="quantity">{line?.approved_quantity.toLocaleString() ?? 'Unavailable'}</td><td className="quantity">{line?.remaining_quantity.toLocaleString() ?? 'Unavailable'}</td></tr>;
      })}</Table></section>
    {transaction.status === 'APPROVED' && <p className="context-notice">Approved transactions are immutable.</p>}
    {transaction.status === 'REJECTED_BY_SYSTEM' && transaction.review_status === 'RESOLVED' && <p className="context-notice">This rejected review is resolved. It did not approve or post excess stock; a new receiving transaction is allowed while the document remains incomplete.</p>}
    {supervisor && (transaction.status === 'PENDING_APPROVAL' || unresolved) && <section className="form-card"><h2>Supervisor actions</h2><p className="section-description">Review the recorded quantities before deciding. Expected quantity limits cannot be overridden.</p><div className="form-actions">
      {transaction.status === 'PENDING_APPROVAL' && <Button disabled={mutation.isPending} onClick={() => setConfirming(true)}>Approve transaction</Button>}
      {unresolved && <><Button disabled={mutation.isPending} onClick={() => { setForm('correct'); setError(''); }}>Correct recorded input</Button><Button variant="secondary" disabled={mutation.isPending} onClick={() => { setForm('review'); setError(''); }}>Resolve confirmed excess</Button></>}
    </div>
      {unresolved && form === 'correct' && <div className="action-form"><h3>Correct recorded input</h3><ActualQuantityForm key={transaction.updated_at} receiving={document.data} transaction={transaction} busy={mutation.isPending} onCancel={() => setForm(null)} onSubmit={(items, reason) => change({ type: 'correct', items, reason })} /></div>}
      {unresolved && form === 'review' && <form className="action-form" aria-label="Resolve confirmed physical excess" noValidate onSubmit={review}><h3>Resolve confirmed physical excess</h3><p className="section-description">Use this when the excess was physically confirmed. The transaction stays rejected, inventory is unchanged, and the active slot is released.</p><TextField id="verification-note" label="Verification note" required value={note} error={noteError} disabled={mutation.isPending} onChange={event => { setNote(event.target.value); setNoteError(''); }} /><div className="form-actions"><Button type="submit" disabled={mutation.isPending}>{mutation.isPending ? 'Resolving…' : 'Resolve review'}</Button><Button variant="secondary" disabled={mutation.isPending} onClick={() => setForm(null)}>Cancel</Button></div></form>}
    </section>}
    {supervisor && confirming && transaction.status === 'PENDING_APPROVAL' && <ConfirmDialog title="Approve receiving transaction?" busy={mutation.isPending} onCancel={() => setConfirming(false)} onConfirm={() => { void change({ type: 'approve' }); }}><p>Approve these recorded quantities and post their inbound inventory once. Every SKU must remain within its expected receiving limit. Approved transactions cannot be edited.</p><ul>{transaction.items.map(item => <li key={item.id}>{receiving.items.find(line => line.id === item.receiving_document_item_id)?.sku ?? item.receiving_document_item_id}: {item.actual_quantity.toLocaleString()}</li>)}</ul></ConfirmDialog>}
  </>;
}
