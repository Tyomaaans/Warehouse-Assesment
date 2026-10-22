import { useState, useRef, type FormEvent } from 'react';
import { ArrowRight, Eye, EyeOff, Layers3, LockKeyhole } from 'lucide-react';
import { Navigate, useLocation } from 'react-router-dom';
import { Brand } from '../../components/Brand';
import { Button } from '../../components/ui/Button';
import { TextField } from '../../components/ui/TextField';
import { LoadingState, ErrorState } from '../../components/ui/States';
import { ApiError } from '../../lib/api/types';
import { useAuth } from './use-auth';

export function LoginPage() {
  const auth = useAuth();
  const location = useLocation();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [remember, setRemember] = useState(false);
  const [visible, setVisible] = useState(false);
  const [pending, setPending] = useState(false);
  const [errors, setErrors] = useState<{ email?: string; password?: string; form?: string }>({});
  const emailRef = useRef<HTMLInputElement>(null);
  const passwordRef = useRef<HTMLInputElement>(null);

  if (auth.status === 'restoring' || auth.signingOut) return <LoadingState label={auth.signingOut ? 'Signing out…' : 'Restoring your session…'} />;
  if (auth.status === 'unavailable') return <div className="session-gate"><ErrorState title="Your session is temporarily unavailable" action={<Button onClick={auth.restore}>Try again</Button>}>{auth.message}</ErrorState></div>;
  if (auth.user) {
    const from = (location.state as { from?: string } | null)?.from;
    return <Navigate to={from && from.startsWith('/') && !from.startsWith('//') && from !== '/login' ? from : '/'} replace />;
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const next: typeof errors = {};
    if (!email.trim()) next.email = 'Enter your email address.';
    else if (!emailRef.current?.validity.valid) next.email = 'Enter a valid email address.';
    if (!password) next.password = 'Enter your password.';
    setErrors(next);
    if (next.email || next.password) { (next.email ? emailRef : passwordRef).current?.focus(); return; }
    setPending(true);
    try { await auth.login({ email: email.trim(), password, remember_me: remember }); }
    catch (error) { setErrors({ form: error instanceof ApiError ? error.message : 'Sign in could not be completed. Please try again.' }); }
    finally { setPending(false); }
  }

  return <div className="login-page">
    <aside className="login-story" aria-label="About this workspace">
      <Brand />
      <div className="story-copy"><span className="story-label"><Layers3 size={15} aria-hidden="true" /> A little more order.</span><h1>Good receiving<br />starts here<span>.</span></h1><p>A clear space for what’s coming in.<br />From the first receipt to the final approval.</p></div>
      <div className="warehouse-art" aria-hidden="true"><div className="art-grid" /><div className="warehouse-frame"><div className="warehouse-roof" /><div className="warehouse-bay"><i /><i /><i /></div><div className="art-box box-one" /><div className="art-box box-two" /><div className="art-box box-three" /></div><span className="art-caption">EVERY RECEIPT. ACCOUNTED FOR.</span></div>
      <p className="story-footer">Expected <ArrowRight size={13} aria-hidden="true" /> Received <ArrowRight size={13} aria-hidden="true" /> Approved</p>
    </aside>
    <main className="login-main" id="main-content">
      <div className="mobile-brand"><Brand compact /></div>
      <div className="login-form-wrap"><span className="login-icon"><LockKeyhole size={21} strokeWidth={1.6} aria-hidden="true" /></span><p className="eyebrow">YOUR WORKSPACE AWAITS</p><h2>Welcome back.</h2><p className="login-description">Sign in to your warehouse workspace.</p>
        {auth.logoutNotice && <div className="form-alert" role="alert"><p>{auth.logoutNotice}</p><Button variant="secondary" onClick={() => { void auth.logout(); }}>Retry sign out</Button></div>}
        <form noValidate onSubmit={event => { void submit(event); }} aria-label="Sign in" aria-busy={pending}>
          {errors.form && <p className="form-alert" role="alert">{errors.form}</p>}
          <TextField id="email" ref={emailRef} label="Email address" type="email" autoComplete="username" inputMode="email" value={email} onChange={event => setEmail(event.target.value)} error={errors.email} required disabled={pending} placeholder="you@your-company.com" />
          <div className="password-field"><TextField id="password" ref={passwordRef} label="Password" type={visible ? 'text' : 'password'} autoComplete="current-password" value={password} onChange={event => setPassword(event.target.value)} error={errors.password} required disabled={pending} placeholder="Enter your password" /><button className="password-toggle" type="button" aria-label={visible ? 'Hide password' : 'Show password'} aria-pressed={visible} onClick={() => setVisible(!visible)}>{visible ? <EyeOff size={18} /> : <Eye size={18} />}</button></div>
          <label className="checkbox-label"><input type="checkbox" checked={remember} onChange={event => setRemember(event.target.checked)} disabled={pending} /> Keep me signed in</label>
          <Button type="submit" className="login-submit" disabled={pending}>{pending ? 'Signing in…' : 'Sign in'}<ArrowRight size={17} aria-hidden="true" /></Button>
        </form>
        <p className="login-help">Need access? Contact your warehouse administrator.</p>
      </div>
      <p className="login-footer">A place for everything coming in.</p>
    </main>
  </div>;
}
