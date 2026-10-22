import { ArrowDownToLine, ArrowRight, Check, ClipboardCheck, ClipboardList, CornerDownRight } from 'lucide-react';
import { Link } from 'react-router-dom';
import { PageHeader } from '../../components/ui/PageHeader';
import { areas, roleArea } from '../../app/navigation';
import { useAuth } from '../auth/use-auth';
import { roleLabel } from '../auth/session';

export function OverviewPage() {
  const { user } = useAuth();
  if (!user) return null;
  const entry = roleArea(user.role);
  return <>
    <PageHeader eyebrow="A LITTLE MORE CLARITY" title="Your warehouse workspace." description="Everything coming in, with a clear path forward." />
    <section className="welcome-card"><div className="welcome-copy"><span className="small-tag"><span className="tiny-dot" /> YOUR SPACE TO WORK</span><h2>Make room for<br />a smoother day.</h2><p>Plan what’s expected. Record what arrives.<br className="desktop-break" /> Keep every approval accounted for.</p><Link className="button button-primary" to={entry.path}>{user.role === 'SUPERVISOR' ? 'Open review workspace' : 'Open receiving documents'}<ArrowRight size={16} aria-hidden="true" /></Link></div><div className="workflow-visual" aria-label="Receiving workflow"><div className="workflow-step"><span><ClipboardList size={19} /></span><div><strong>Set expectations</strong><small>Expected SKU quantities</small></div></div><CornerDownRight className="workflow-connector" size={20} aria-hidden="true" /><div className="workflow-step"><span><ArrowDownToLine size={19} /></span><div><strong>Record the receipt</strong><small>Actual physical quantities</small></div></div><CornerDownRight className="workflow-connector" size={20} aria-hidden="true" /><div className="workflow-step final"><span><ClipboardCheck size={19} /></span><div><strong>Review & approve</strong><small>Approved receipts update inventory</small></div><Check size={17} aria-hidden="true" /></div></div></section>
    <div className="section-heading"><div><p className="eyebrow">FIND YOUR WAY</p><h2>A place for each step.</h2></div><span className="subtle-label">Workspace guide</span></div>
    <div className="area-grid">{areas.filter(area => area.path !== '/').map(area => <Link className="area-card" key={area.path} to={area.path}><span className="area-icon"><area.icon size={21} strokeWidth={1.6} aria-hidden="true" /></span><ArrowRight className="area-arrow" size={17} aria-hidden="true" /><h3>{area.label}</h3><p>{area.description}</p><span className="area-label">View workspace <span>→</span></span></Link>)}</div>
    <section className="access-note"><span className="access-icon"><ClipboardCheck size={20} aria-hidden="true" /></span><div><h2>Working as {roleLabel(user.role)}</h2><p>{user.role === 'SUPERVISOR' ? 'Your role can manage products, review rejected receipts and approve valid receiving transactions.' : 'Your role can create receiving documents and record physical receipts. Supervisor approval is required before inventory changes.'}</p></div><span className="pill">{roleLabel(user.role)} access</span></section>
  </>;
}
