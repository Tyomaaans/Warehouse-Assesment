import { Package } from 'lucide-react';

export function Brand({ compact = false }: { compact?: boolean }) {
  return <div className="brand"><span className="brand-icon"><Package size={23} strokeWidth={1.7} aria-hidden="true" /></span><span>stow<span className="brand-dot">.</span>{!compact && <small>WAREHOUSE WORKSPACE</small>}</span></div>;
}
