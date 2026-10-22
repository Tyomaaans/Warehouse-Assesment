import { useRef, useState, type FormEvent } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from 'react-router-dom';
import { Plus, Trash2 } from 'lucide-react';
import { queryKeys } from '../../app/query-client';
import { Button } from '../../components/ui/Button';
import { TextField } from '../../components/ui/TextField';
import { PageHeader } from '../../components/ui/PageHeader';
import { EmptyState, LoadingState } from '../../components/ui/States';
import { Pagination, QueryError } from '../../components/ui/ListControls';
import { focusInvalid } from '../../lib/view';
import { useAuth } from '../auth/use-auth';
import { productsApi, type Product } from '../products/api';
import { isCanonicalID, positiveQuantity, receivingApi } from './api';

interface Line { key: number; product: Product | null; quantity: string }
interface LineErrors { product?: string; quantity?: string }

export function CreateReceivingPage() {
  const { user } = useAuth();
  const cache = useQueryClient();
  const navigate = useNavigate();
  const nextKey = useRef(1);
  const [lines, setLines] = useState<Line[]>([{ key: 0, product: null, quantity: '' }]);
  const [orderReference, setOrderReference] = useState('');
  const [orderError, setOrderError] = useState('');
  const [lineErrors, setLineErrors] = useState<Record<number, LineErrors>>({});
  const [lookup, setLookup] = useState({ page: 1, limit: 10, search: '' });
  const products = useQuery({ queryKey: [...queryKeys.products(user!.id), lookup], queryFn: ({ signal }) => productsApi.list(lookup, signal) });
  const mutation = useMutation({ mutationFn: receivingApi.create, onSuccess: async document => {
    await cache.invalidateQueries({ queryKey: queryKeys.receivings(user!.id) });
    cache.setQueryData([...queryKeys.receivings(user!.id), 'detail', document.id], document);
    navigate(`/receivings/${document.id}`, { replace: true, state: { created: true } });
  } });
  if (user?.role !== 'OPERATOR') return null;

  function update(key: number, change: Partial<Line>) {
    setLines(previous => previous.map(line => line.key === key ? { ...line, ...change } : line));
    setLineErrors(previous => ({ ...previous, [key]: { ...previous[key], ...(change.quantity !== undefined ? { quantity: undefined } : { product: undefined }) } }));
    if (mutation.error) mutation.reset();
  }
  function addLine() {
    const key = nextKey.current++;
    setLines([...lines, { key, product: null, quantity: '' }]);
    requestAnimationFrame(() => document.getElementById(`line-product-${key}`)?.focus());
  }
  function removeLine(key: number) {
    const remaining = lines.filter(item => item.key !== key);
    setLines(remaining);
    setLineErrors({});
    requestAnimationFrame(() => document.getElementById(`line-product-${remaining[remaining.length - 1]!.key}`)?.focus());
  }
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (mutation.isPending) return;
    const form = event.currentTarget;
    const next: Record<number, LineErrors> = {};
    const used = new Set<string>();
    for (const line of lines) {
      const error: LineErrors = {};
      if (!line.product || !isCanonicalID(line.product.id)) error.product = 'Choose a product for this line.';
      else if (used.has(line.product.id)) error.product = 'Each SKU can appear only once.';
      if (line.product) used.add(line.product.id);
      if (positiveQuantity(line.quantity) === null) error.quantity = 'Enter a whole number from 1 to 9223372036854775807.';
      if (error.product || error.quantity) next[line.key] = error;
    }
    const reference = orderReference.trim();
    setOrderError(reference ? '' : 'Enter the external ORD reference.');
    setLineErrors(next);
    mutation.reset();
    if (!reference || Object.keys(next).length) { focusInvalid(form); return; }
    mutation.mutate({ order_reference: reference, items: lines.map(line => ({ product_id: line.product!.id, expected_quantity: positiveQuantity(line.quantity)! })) });
  }

  return <><Link className="back-link" to="/receivings">← Receiving documents</Link><PageHeader eyebrow="SET EXPECTATIONS" title="New receiving document" description="Reference an external order and record the SKU quantities you expect to receive." />
    <section className="form-card"><div className="snapshot-note"><strong>Expected quantities are saved once.</strong><p>Creating this document sets its expected quantities. It starts incomplete; inventory changes only after a Supervisor approves a receiving transaction.</p></div>
      <div className="catalogue-picker"><h2>Find products for your SKU lines</h2><p className="section-description">Search the catalogue, then choose products below. Selected products stay selected when you change catalogue pages.</p><form className="search-form" role="search" onSubmit={event => { event.preventDefault(); setLookup({ ...lookup, page: 1, search: String(new FormData(event.currentTarget).get('search') ?? '') }); }}><label className="sr-only" htmlFor="sku-search">Search available products</label><input id="sku-search" name="search" type="search" maxLength={100} placeholder="Search SKU or product name" /><Button type="submit" variant="secondary">Search products</Button></form>
        {products.isPending ? <LoadingState label="Loading available products…" /> : products.isError ? <QueryError error={products.error} retry={() => { void products.refetch(); }} /> : <>{!products.data.products.length && <EmptyState title="No available products on this page">Try another search or catalogue page. A Supervisor can create missing products.</EmptyState>}<Pagination meta={products.data.meta} onPage={page => setLookup({ ...lookup, page })} busy={products.isFetching} /></>}
      </div>
      <form noValidate onSubmit={submit} aria-label="Create receiving document" aria-busy={mutation.isPending}>
        {mutation.error && <p className="form-alert" role="alert">{mutation.error.message}</p>}
        <TextField id="order-reference" label="External ORD reference" value={orderReference} onChange={event => { setOrderReference(event.target.value); setOrderError(''); if (mutation.error) mutation.reset(); }} required error={orderError} disabled={mutation.isPending} />
        <div className="lines-heading"><h2>Expected SKU lines</h2><span>One line per SKU</span></div>
        {lines.map((line, index) => {
          const available = products.data?.products ?? [];
          const choices = [...new Map([...available, ...lines.flatMap(item => item.product ? [item.product] : [])].map(product => [product.id, product])).values()];
          const error = lineErrors[line.key];
          return <fieldset className="sku-line" key={line.key} disabled={mutation.isPending}><legend>SKU line {index + 1}</legend><div className="sku-line-fields"><div className="field"><label htmlFor={`line-product-${line.key}`}>Product for line {index + 1}</label><select id={`line-product-${line.key}`} required value={line.product?.id ?? ''} aria-invalid={error?.product ? true : undefined} aria-describedby={error?.product ? `line-product-${line.key}-error` : undefined} onChange={event => update(line.key, { product: choices.find(product => product.id === event.target.value) ?? null })}><option value="">Choose a product</option>{choices.map(product => <option key={product.id} value={product.id} disabled={lines.some(other => other.key !== line.key && other.product?.id === product.id)}>{product.sku} — {product.name}</option>)}</select>{error?.product && <p className="field-error" id={`line-product-${line.key}-error`}>{error.product}</p>}</div><TextField id={`line-quantity-${line.key}`} label={`Expected quantity for line ${index + 1}`} type="text" inputMode="numeric" value={line.quantity} onChange={event => update(line.key, { quantity: event.target.value })} error={error?.quantity} required /><Button variant="quiet" aria-label={`Remove SKU line ${index + 1}`} disabled={lines.length === 1 || mutation.isPending} onClick={() => removeLine(line.key)}><Trash2 size={17} aria-hidden="true" />Remove</Button></div></fieldset>;
        })}
        <Button variant="secondary" disabled={mutation.isPending} onClick={addLine}><Plus size={16} aria-hidden="true" />Add SKU line</Button>
        <div className="form-actions receiving-submit"><Button type="submit" disabled={mutation.isPending}>{mutation.isPending ? 'Creating document…' : 'Create receiving document'}</Button><Link className="button button-secondary" to="/receivings" aria-disabled={mutation.isPending} onClick={event => { if (mutation.isPending) event.preventDefault(); }}>Cancel</Link></div>
      </form>
    </section>
  </>;
}
