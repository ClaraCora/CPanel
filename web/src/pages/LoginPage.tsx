import { useState, type FormEvent } from "react";
import { Eye, EyeOff, LockKeyhole, Mail } from "lucide-react";
import { ApiError, authApi, demoMode } from "../auth";
import { Button, Field } from "../components/ui";
import { ThemeToggle } from "../components/ThemeToggle";
import type { Session } from "../types";

export function LoginPage({ initialError = "", onLogin }: { initialError?: string; onLogin: (session: Session) => void }) {
  const [email, setEmail] = useState(demoMode ? "admin@cpanel.local" : "");
  const [password, setPassword] = useState(demoMode ? "demo-password" : "");
  const [visible, setVisible] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(initialError);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setLoading(true); setError("");
    try { onLogin(await authApi.login(email.trim(), password)); }
    catch (reason) { setError(reason instanceof ApiError ? reason.message : "无法连接管理服务，请确认服务已启动"); }
    finally { setLoading(false); }
  }

  return <main className="login-page"><ThemeToggle className="theme-toggle--floating" /><section className="login-panel" aria-labelledby="login-title"><div className="login-brand"><span className="brand__mark">C</span><div><strong>CPanel</strong><span>设备管理平台</span></div></div><div className="login-copy"><h1 id="login-title">管理员登录</h1><p>只有管理员账号可以登录管理后台。</p></div>{error && <div className="form-alert" role="alert">{error}</div>}<form onSubmit={submit} className="login-form"><Field label="邮箱" required><div className="input-with-icon"><Mail size={17} /><input type="email" autoComplete="username" value={email} onChange={(event) => setEmail(event.target.value)} required autoFocus /></div></Field><Field label="密码" required><div className="input-with-icon"><LockKeyhole size={17} /><input type={visible ? "text" : "password"} autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} required /><button type="button" className="password-toggle" aria-label={visible ? "隐藏密码" : "显示密码"} title={visible ? "隐藏密码" : "显示密码"} onClick={() => setVisible((value) => !value)}>{visible ? <EyeOff size={17} /> : <Eye size={17} />}</button></div></Field><Button variant="primary" loading={loading} type="submit">登录后台</Button></form><footer>会话使用 HttpOnly Cookie 与 CSRF 双重保护</footer></section></main>;
}
