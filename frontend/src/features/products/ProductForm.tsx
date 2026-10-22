import { useState, type FormEvent } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Button } from '../../components/ui/Button';
import { TextField } from '../../components/ui/TextField';
import { focusInvalid } from '../../lib/view';
import { queryKeys } from '../../app/query-client';
import { useAuth } from '../auth/use-auth';
import { productsApi, type Product } from './api';

export function ProductForm({ onCreated, onCancel }: { onCreated: (product: Product) => void; onCancel: () => void }) {
  const { user } = useAuth();
  const cache = useQueryClient();
  const [errors, setErrors] = useState<{ sku?: string; name?: string }>({});
  const mutation = useMutation({ mutationFn: productsApi.create, onSuccess: async product => {
    await cache.invalidateQueries({ queryKey: queryKeys.products(user!.id) });
    onCreated(product);
  } });
  if (user?.role !== 'SUPERVISOR') return null;

  function edited(field: 'sku' | 'name') {
    setErrors(previous => ({ ...previous, [field]: undefined }));
    if (mutation.error) mutation.reset();
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (mutation.isPending) return;
    const form = event.currentTarget;
    const data = new FormData(form);
    const sku = String(data.get('sku') ?? '').trim();
    const name = String(data.get('name') ?? '').trim();
    const next = { sku: sku ? undefined : 'Enter a SKU.', name: name ? undefined : 'Enter a product name.' };
    setErrors(next);
    mutation.reset();
    if (next.sku || next.name) { focusInvalid(form); return; }
    mutation.mutate({ sku, name });
  }

  return <section className="form-card" aria-labelledby="product-form-title"><h2 id="product-form-title">Create product</h2><p className="section-description">Use a unique SKU so every receiving line identifies the right product.</p>
    <form noValidate onSubmit={submit} aria-label="Create product" aria-busy={mutation.isPending}>
      {mutation.error && <p className="form-alert" role="alert">{mutation.error.message}</p>}
      <div className="form-grid"><TextField id="product-sku" name="sku" label="SKU" error={errors.sku} onChange={() => edited('sku')} required disabled={mutation.isPending} autoFocus /><TextField id="product-name" name="name" label="Product name" error={errors.name} onChange={() => edited('name')} required disabled={mutation.isPending} /></div>
      <div className="form-actions"><Button type="submit" disabled={mutation.isPending}>{mutation.isPending ? 'Creating product…' : 'Create product'}</Button><Button variant="secondary" onClick={onCancel} disabled={mutation.isPending}>Cancel</Button></div>
    </form>
  </section>;
}
