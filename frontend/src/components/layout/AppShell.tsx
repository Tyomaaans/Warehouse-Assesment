import { useRef, useEffect } from 'react';
import { NavLink, Outlet, useLocation, Link } from 'react-router-dom';
import { ArrowUpRight, ChevronRight, LogOut, Menu, ShieldCheck, X } from 'lucide-react';
import { Brand } from '../Brand';
import { Button } from '../ui/Button';
import { areas, reviewArea } from '../../app/navigation';
import { useAuth } from '../../features/auth/use-auth';
import { roleLabel } from '../../features/auth/session';

export function AppShell() {
  const auth = useAuth();
  const location = useLocation();
  const dialog = useRef<HTMLDialogElement>(null);
  const main = useRef<HTMLElement>(null);
  const area = [reviewArea, ...areas].find(item => item.path === location.pathname);

  useEffect(() => {
    dialog.current?.close();
    window.scrollTo(0, 0);
    main.current?.focus({ preventScroll: true });
    document.title = `${area?.label ?? 'Page not found'} — Stow`;
  }, [location.pathname, area?.label]);

  if (!auth.user) return null;
  const user = auth.user;
  const navigation = <>
    <Brand />
    <p className="nav-label">WORKSPACE</p>
    <nav aria-label="Main navigation">{areas.map(item => <NavLink key={item.path} to={item.path} end className={({ isActive }) => `nav-link${isActive ? ' active' : ''}`}><item.icon size={18} strokeWidth={1.7} aria-hidden="true" /><span>{item.label}</span></NavLink>)}</nav>
    {user.role === 'SUPERVISOR' && <><p className="nav-label role-nav-label">YOUR ROLE</p><nav aria-label="Supervisor navigation"><NavLink to={reviewArea.path} className={({ isActive }) => `nav-link${isActive ? ' active' : ''}`}><ShieldCheck size={18} aria-hidden="true" /><span>Supervisor review</span></NavLink></nav></>}
    <div className="sidebar-bottom"><div className="workspace-note"><span className="tiny-dot" /><span>One warehouse.<br /><strong>One clear picture.</strong></span></div><div className="account"><div className="avatar">{user.role === 'SUPERVISOR' ? 'SP' : 'OP'}</div><div><strong>{roleLabel(user.role)}</strong><span title={user.email}>{user.email ?? 'Warehouse session'}</span></div></div><Button variant="quiet" className="logout-button" onClick={() => { void auth.logout(); }} disabled={auth.signingOut}><LogOut size={17} aria-hidden="true" /> Sign out</Button></div>
  </>;

  return <div className="app-shell">
    <a className="skip-link" href="#main-content">Skip to content</a>
    <aside className="sidebar">{navigation}</aside>
    <dialog ref={dialog} className="mobile-navigation" aria-label="Navigation"><div className="drawer-close"><Button variant="quiet" aria-label="Close navigation" onClick={() => dialog.current?.close()}><X size={21} /></Button></div>{navigation}</dialog>
    <div className="main-column"><header className="topbar"><div className="breadcrumb"><Button className="menu-button" variant="quiet" aria-label="Open navigation" onClick={() => dialog.current?.showModal()}><Menu size={21} /></Button><span>Workspace</span><ChevronRight size={14} aria-hidden="true" /><strong>{area?.label ?? 'Page not found'}</strong></div><span className="role-badge"><ShieldCheck size={14} aria-hidden="true" />{roleLabel(user.role)}</span></header>
      <main id="main-content" className="main-content" ref={main} tabIndex={-1}><Outlet /></main>
      <footer className="app-footer"><span>Stow · Warehouse workspace</span><Link to="/">Back to overview <ArrowUpRight size={13} aria-hidden="true" /></Link></footer>
    </div>
  </div>;
}
