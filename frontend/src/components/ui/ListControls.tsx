import type { PaginationMeta } from '../../lib/api/types';
import { ApiError } from '../../lib/api/types';
import { Button } from './Button';
import { ErrorState } from './States';

export function Pagination({ meta, onPage, busy = false }: { meta: PaginationMeta; onPage: (page: number) => void; busy?: boolean }) {
  return <nav className="pagination" aria-label="Pagination">
    <p>{meta.total_data.toLocaleString()} {meta.total_data === 1 ? 'record' : 'records'} · Page {meta.current_page}{meta.total_pages > 0 ? ` of ${meta.total_pages}` : ''}</p>
    <div><Button variant="secondary" disabled={busy || meta.current_page <= 1} onClick={() => onPage(meta.current_page - 1)}>Previous</Button><Button variant="secondary" disabled={busy || meta.current_page >= meta.total_pages || meta.current_page >= 10000} onClick={() => onPage(meta.current_page + 1)}>Next</Button></div>
  </nav>;
}

export function QueryError({ error, retry }: { error: Error; retry: () => void }) {
  const title = error instanceof ApiError && error.status === 403 ? 'Access denied' : error instanceof ApiError && error.status === 404 ? 'Document not found' : 'Unable to load this view';
  return <ErrorState title={title} action={<Button variant="secondary" onClick={retry}>Try again</Button>}>{error.message}</ErrorState>;
}

export function PageSize({ value, onChange }: { value: number; onChange: (limit: number) => void }) {
  return <label className="inline-field">Rows per page<select value={value} onChange={event => onChange(Number(event.target.value))}>{[10, 25, 50, 100].map(size => <option key={size} value={size}>{size}</option>)}</select></label>;
}
