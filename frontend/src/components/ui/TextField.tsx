import type { ComponentProps } from 'react';

export function TextField({ id, label, error, ...props }: ComponentProps<'input'> & { id: string; label: string; error?: string }) {
  return <div className="field">
    <label htmlFor={id}>{label}</label>
    <input id={id} aria-invalid={error ? true : undefined} aria-describedby={error ? `${id}-error` : undefined} {...props} />
    {error && <p id={`${id}-error`} className="field-error">{error}</p>}
  </div>;
}
