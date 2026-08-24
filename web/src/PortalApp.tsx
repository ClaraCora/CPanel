import { lazy, Suspense, useEffect, useState, type FormEvent } from "react";
import { Eye, EyeOff, TriangleAlert } from "lucide-react";
import { ApiError, demoMode, portalSessionExpiredEvent } from "./auth";
import { portalApi } from "./portalApi";
import { ThemeToggle } from "./components/ThemeToggle";
import type { PortalSession } from "./types";
import "./portal.css";

const PortalDashboardPage = lazy(() => import("./PortalDashboardPage"));

export default function PortalApp() {
  const [session, setSession] = useState<PortalSession | null>(null);
  const [checking, setChecking] = useState(!demoMode);
  const [entryError, setEntryError] = useState("");

  useEffect(() => {
    let active = true;
    const grant = new URLSearchParams(window.location.search).get("grant");
    const bootstrap = grant ? portalApi.redeem(grant) : portalApi.current();
    bootstrap.then((value) => {
      if (!active) return;
      setSession(value);
      if (grant) window.history.replaceState(null, "", "/edu");
    }).catch((reason) => {
      if (!active) return;
      setSession(null);
      if (grant) {
        setEntryError(reason instanceof ApiError ? reason.message : "管理员授权已失效，请返回后台重新进入");
        window.history.replaceState(null, "", "/edu");
      }
    }).finally(() => { if (active) setChecking(false); });
    return () => { active = false; };
  }, []);

  useEffect(() => {
    const expireSession = () => {
      setSession(null);
      setEntryError("登录状态已过期，请重新登录");
      window.history.replaceState(null, "", "/edu");
    };
    window.addEventListener(portalSessionExpiredEvent, expireSession);
    return () => window.removeEventListener(portalSessionExpiredEvent, expireSession);
  }, []);

  if (checking) return <main className="edu-boot"><ThemeToggle className="theme-toggle--floating edu-theme-toggle" /><div className="edu-boot__mark">C</div><span>正在验证登录状态…</span></main>;
  if (!session) return <PortalLogin initialError={entryError} onLogin={setSession} />;
  return <Suspense fallback={<main className="edu-boot"><div className="edu-boot__mark">C</div><span>正在准备页面…</span></main>}><PortalDashboardPage session={session} onLogout={() => setSession(null)} /></Suspense>;
}

function PortalLogin({ initialError, onLogin }: { initialError: string; onLogin: (session: PortalSession) => void }) {
  const [login, setLogin] = useState(demoMode ? "li.ming@example.com" : "");
  const [password, setPassword] = useState(demoMode ? "demo-password" : "");
  const [visible, setVisible] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(initialError);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setLoading(true); setError("");
    try { onLogin(await portalApi.login(login.trim(), password)); }
    catch (reason) { setError(reason instanceof ApiError ? reason.message : "无法连接服务，请稍后重试"); }
    finally { setLoading(false); }
  }

  return <main className="edu-login"><ThemeToggle className="edu-theme-toggle" /><section className="edu-login__panel"><div className="edu-login__brand"><span>C</span><strong>设备管理平台</strong></div>{error && <div className="edu-alert" role="alert"><TriangleAlert size={17} />{error}</div>}<form onSubmit={submit}><label>门户账号<input autoComplete="username" value={login} onChange={(event) => setLogin(event.target.value)} placeholder="账号或邮箱" required autoFocus /></label><label>密码<span className="edu-password"><input type={visible ? "text" : "password"} autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} required /><button type="button" aria-label={visible ? "隐藏密码" : "显示密码"} onClick={() => setVisible((value) => !value)}>{visible ? <EyeOff size={17} /> : <Eye size={17} />}</button></span></label><button className="edu-button edu-button--primary" disabled={loading} type="submit">{loading ? "登录中…" : "登录"}</button></form></section></main>;
}
