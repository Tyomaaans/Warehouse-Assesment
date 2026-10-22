import { Link } from 'react-router-dom';
import { PageHeader } from '../components/ui/PageHeader';
import { EmptyState } from '../components/ui/States';

export function FeaturePlaceholder({ label, description, planned }: { label: string; description: string; planned: string }) {
  return <><PageHeader eyebrow="WORKSPACE" title={label} description={description} /><div className="placeholder-card"><span className="pill">Planned view</span><EmptyState title="This workspace is being prepared.">{planned} This page is a placeholder; no live records are displayed.</EmptyState><Link to="/" className="text-link">← Back to overview</Link></div></>;
}
