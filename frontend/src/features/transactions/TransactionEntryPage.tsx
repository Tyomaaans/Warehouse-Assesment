import { useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { queryKeys } from '../../app/query-client';
import { PageHeader } from '../../components/ui/PageHeader';
import { ErrorState, LoadingState } from '../../components/ui/States';
import { QueryError } from '../../components/ui/ListControls';
import { useAuth } from '../auth/use-auth';
import { isCanonicalID, receivingApi } from '../receiving/api';
import { ActualQuantityForm } from './ActualQuantityForm';
import { isActive, transactionsApi, type ActualItem } from './api';
import { refreshReceiving } from './queries';

export function TransactionEntryPage() {
  const { id = '' } = useParams();
  const { user } = useAuth();
  const cache = useQueryClient();
  const navigate = useNavigate();
  const [error, setError] = useState('');
  const lock = useRef(false);
  const valid = isCanonicalID(id);
  const document = useQuery({ queryKey: [...queryKeys.receivings(user!.id), 'detail', id], queryFn: ({ signal }) => receivingApi.detail(id, signal), enabled: valid, staleTime: 0 });
  // Sequential receiving makes any active transaction the newest one. The backend rechecks under lock.
  const latest = useQuery({ queryKey: [...queryKeys.transactions(user!.id, id), 1, 1], queryFn: ({ signal }) => transactionsApi.list(id, 1, 1, signal), enabled: valid, staleTime: 0 });
  const mutation = useMutation({ mutationFn: (items: ActualItem[]) => transactionsApi.create(id, items), onSuccess: async transaction => {
    await refreshReceiving(cache, user!.id, id, transaction);
    navigate(`/transactions/${transaction.id}`, { replace: true, state: { recorded: true } });
  }, onError: async failure => { setError(failure.message); await refreshReceiving(cache, user!.id, id); } });
  if (!valid) return <ErrorState title="Invalid document identifier">Open a receiving document from the list.</ErrorState>;
  if (document.isPending || latest.isPending) return <LoadingState label="Loading receiving context…" />;
  if (document.isError || latest.isError) return <QueryError error={(document.error ?? latest.error)!} retry={() => { void document.refetch(); void latest.refetch(); }} />;
  const active = latest.data.transactions.find(isActive);
  async function submit(items: ActualItem[]) {
    if (lock.current || user?.role !== 'OPERATOR') return;
    lock.current = true; setError('');
    try { await mutation.mutateAsync(items); } catch { /* Visible error and live context supplied by onError. */ }
    finally { lock.current = false; }
  }
  return <><Link className="back-link" to={`/receivings/${id}`}>← Receiving document</Link><PageHeader eyebrow="ACTUAL PHYSICAL RECEIVING" title="Record physical receipt" description={`External ORD: ${document.data.order_reference}`} />
    {error && <p role="alert" className="form-alert">{error} Check the current receiving state before retrying.</p>}
    {document.data.status === 'COMPLETED' ? <ErrorState title="Receiving is completed">This document is closed to new transactions.</ErrorState> : active ? <ErrorState title="An active transaction already exists" action={<Link className="text-link" to={`/transactions/${active.id}`}>Open active transaction</Link>}>Wait for approval or resolution of the unresolved rejection before recording another receipt.</ErrorState> : <section className="form-card"><ActualQuantityForm receiving={document.data} busy={mutation.isPending} onSubmit={submit} /></section>}
  </>;
}
