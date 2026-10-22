import { AlertCircle, LoaderCircle, PackageOpen } from 'lucide-react';
import type { ReactNode } from 'react';

export function LoadingState({ label = 'Loading your workspace…' }: { label?: string }) {
  return <div className="state-panel" role="status"><LoaderCircle className="spinner" size={25} aria-hidden="true" /><p>{label}</p></div>;
}
export function ErrorState({ title, children, action }: { title: string; children: ReactNode; action?: ReactNode }) {
  return <section className="state-panel" role="alert"><span className="state-icon error"><AlertCircle size={25} aria-hidden="true" /></span><h2>{title}</h2><p>{children}</p>{action}</section>;
}
export function EmptyState({ title, children }: { title: string; children: ReactNode }) {
  return <section className="state-panel"><span className="state-icon"><PackageOpen size={29} aria-hidden="true" /></span><h2>{title}</h2><p>{children}</p></section>;
}
