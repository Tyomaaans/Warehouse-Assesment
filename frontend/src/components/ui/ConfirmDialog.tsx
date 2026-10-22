import { useEffect, useRef, type ReactNode } from 'react';
import { Button } from './Button';

export function ConfirmDialog({ title, children, busy, onCancel, onConfirm }: { title: string; children: ReactNode; busy: boolean; onCancel: () => void; onConfirm: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => { const node = dialog.current; node?.showModal(); return () => { node?.close(); }; }, []);
  return <dialog className="confirm-dialog" ref={dialog} aria-labelledby="confirmation-title" aria-describedby="confirmation-description" onCancel={event => { event.preventDefault(); if (!busy) onCancel(); }}>
    <h2 id="confirmation-title">{title}</h2><div id="confirmation-description">{children}</div>
    <div className="form-actions"><Button variant="secondary" onClick={onCancel} disabled={busy} autoFocus>Cancel</Button><Button onClick={onConfirm} disabled={busy}>{busy ? 'Approving…' : 'Confirm approval'}</Button></div>
  </dialog>;
}
