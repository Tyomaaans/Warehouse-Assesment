import { useRef, useState, type FormEvent } from 'react';
import { Button } from '../../components/ui/Button';
import { TextField } from '../../components/ui/TextField';
import { focusInvalid } from '../../lib/view';
import { positiveQuantity, type ReceivingDetail } from '../receiving/api';
import type { ActualItem, Transaction } from './api';

export function ActualQuantityForm({ receiving, transaction, busy, onSubmit, onCancel }: {
  receiving: ReceivingDetail; transaction?: Transaction; busy: boolean;
  onSubmit: (items: ActualItem[], reason: string) => Promise<void>; onCancel?: () => void;
}) {
  const [values, setValues] = useState<Record<string, string>>(() => Object.fromEntries(transaction?.items.map(item => [item.receiving_document_item_id, item.actual_quantity.toString()]) ?? []));
  const [selected, setSelected] = useState<Set<string>>(() => new Set(transaction?.items.map(item => item.receiving_document_item_id) ?? []));
  const [reason, setReason] = useState('');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const submitting = useRef(false);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy || submitting.current) return;
    const next: Record<string, string> = {};
    if (!selected.size) next.items = 'Select at least one SKU received.';
    for (const id of selected) if (positiveQuantity(values[id] ?? '') === null) next[id] = 'Enter a whole number from 1 to 9223372036854775807.';
    if (transaction && !reason.trim()) next.reason = 'Enter the reason for this correction.';
    setErrors(next);
    if (Object.keys(next).length) { focusInvalid(event.currentTarget); return; }
    submitting.current = true;
    try { await onSubmit(receiving.items.filter(item => selected.has(item.id)).map(item => ({ receiving_document_item_id: item.id, actual_quantity: positiveQuantity(values[item.id]!)! })), reason.trim()); }
    finally { submitting.current = false; }
  }
  return <form className="actual-form" noValidate onSubmit={event => { void submit(event); }} aria-label={transaction ? 'Correct transaction quantities' : 'Record physical receipt'} aria-busy={busy}>
    <p className="snapshot-note">{transaction ? 'Correct this same transaction. The selected lines replace its recorded SKU list; before/after values and your reason are audited.' : 'Select only the SKUs physically received. Leave other SKUs unchecked; zero-quantity lines are not submitted.'} Quantities above remaining are saved as rejected for Supervisor review; they do not post inventory.</p>
    {errors.items && <p className="form-alert" role="alert">{errors.items}</p>}
    <fieldset disabled={busy} className="actual-lines"><legend className="sr-only">Physical quantities</legend>{receiving.items.map(item => {
      const checked = selected.has(item.id);
      const original = transaction?.items.find(line => line.receiving_document_item_id === item.id);
      const actual = positiveQuantity(values[item.id] ?? '');
      return <section className="actual-line" key={item.id}>
        <label className="checkbox-label"><input type="checkbox" aria-label={`Include ${item.sku}`} checked={checked} aria-invalid={errors.items ? true : undefined} onChange={event => { setSelected(previous => { const next = new Set(previous); if (event.target.checked) next.add(item.id); else next.delete(item.id); return next; }); setErrors(previous => ({ ...previous, items: '' })); }} /><strong>{item.sku}</strong><span>{item.name}</span></label>
        <dl className="quantity-context"><div><dt>Expected</dt><dd>{item.expected_quantity.toLocaleString()}</dd></div><div><dt>Previously approved</dt><dd>{item.approved_quantity.toLocaleString()}</dd></div><div><dt>Remaining</dt><dd>{item.remaining_quantity.toLocaleString()}</dd></div>{transaction && <div><dt>Recorded before correction</dt><dd>{original ? original.actual_quantity.toLocaleString() : 'Not submitted'}</dd></div>}</dl>
        <TextField id={`actual-${item.id}`} label={`${transaction ? 'Corrected' : 'Actual'} quantity for ${item.sku}`} inputMode="numeric" type="text" value={values[item.id] ?? ''} disabled={!checked || busy} required={checked} error={checked ? errors[item.id] : undefined} onChange={event => { setValues(previous => ({ ...previous, [item.id]: event.target.value })); setErrors(previous => ({ ...previous, [item.id]: '' })); }} />
        {checked && actual !== null && actual > item.remaining_quantity && <p className="excess-note">This exceeds the remaining quantity. It can be recorded, but will remain rejected until Supervisor review.</p>}
        {transaction && original && !checked && <p className="excess-note">This recorded line will be omitted from the correction.</p>}
      </section>;
    })}</fieldset>
    {transaction && <TextField id="correction-reason" label="Correction reason" value={reason} required disabled={busy} error={errors.reason} onChange={event => { setReason(event.target.value); setErrors(previous => ({ ...previous, reason: '' })); }} />}
    <div className="form-actions"><Button type="submit" disabled={busy}>{busy ? 'Saving…' : transaction ? 'Save correction' : 'Record physical receipt'}</Button>{onCancel && <Button variant="secondary" disabled={busy} onClick={onCancel}>Cancel</Button>}</div>
  </form>;
}
