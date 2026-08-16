import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { Activity, CalendarDays, Check, CircleUserRound, Clipboard, Copy, Download, Eye, EyeOff, KeyRound, Link2, LogOut, Network, RefreshCw, Server, ShieldCheck, Signal, TriangleAlert, UserRound } from "lucide-react";
import { ApiError, demoMode } from "./auth";
import { portalApi } from "./portalApi";
import type { PortalDashboard, PortalSession } from "./types";
import "./portal.css";

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

  if (checking) return <main className="edu-boot"><div className="edu-boot__mark">C</div><span>正在验证登录状态…</span></main>;
  if (!session) return <PortalLogin initialError={entryError} onLogin={setSession} />;
  return <PortalDashboardPage session={session} onLogout={() => setSession(null)} />;
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
  return <main className="edu-login"><section className="edu-login__panel"><div className="edu-login__brand"><span>C</span><strong>设备管理平台</strong></div>{error && <div className="edu-alert" role="alert"><TriangleAlert size={17} />{error}</div>}<form onSubmit={submit}><label>门户账号<input autoComplete="username" value={login} onChange={(event) => setLogin(event.target.value)} placeholder="账号或邮箱" required autoFocus /></label><label>密码<span className="edu-password"><input type={visible ? "text" : "password"} autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} required /><button type="button" aria-label={visible ? "隐藏密码" : "显示密码"} onClick={() => setVisible((value) => !value)}>{visible ? <EyeOff size={17} /> : <Eye size={17} />}</button></span></label><button className="edu-button edu-button--primary" disabled={loading} type="submit">{loading ? "登录中…" : "登录"}</button></form></section></main>;
}

function PortalDashboardPage({ session, onLogout }: { session: PortalSession; onLogout: () => void }) {
  const [dashboard, setDashboard] = useState<PortalDashboard | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [showReset, setShowReset] = useState(false);

  async function load() {
    setLoading(true); setError("");
    try { setDashboard(await portalApi.dashboard()); }
    catch (reason) { setError(reason instanceof ApiError ? reason.message : "订阅中心加载失败，请稍后重试"); }
    finally { setLoading(false); }
  }
  useEffect(() => { void load(); }, []);

  async function logout() {
    try { await portalApi.logout(); } finally { onLogout(); window.location.reload(); }
  }

  if (loading) return <main className="edu-boot"><div className="edu-boot__mark">C</div><span>正在加载订阅中心…</span></main>;
  if (!dashboard) return <main className="edu-failure"><TriangleAlert size={25} /><h1>订阅中心暂不可用</h1><p>{error}</p><button className="edu-button edu-button--primary" onClick={() => void load()}>重新加载</button></main>;
  const role = dashboard.role === "friend" ? "朋友" : dashboard.role === "admin" ? "管理员" : "用户";
  const traffic = trafficSummary(dashboard.traffic_used_bytes, dashboard.traffic_limit_bytes);
  return <main className="edu-app">
    <header className="edu-topbar">
      <a className="edu-brand" href="/edu"><span>C</span><strong>设备管理平台</strong></a>
      <div className="edu-topbar__account">
        <span className="edu-avatar">{dashboard.name.slice(0, 1).toUpperCase()}</span>
        <div><strong>{dashboard.name}</strong><small>{role}</small></div>
        <button className="edu-icon-button" type="button" title="退出登录" aria-label="退出登录" onClick={() => void logout()}><LogOut size={18} /></button>
      </div>
    </header>
    <div className="edu-shell">
      {session.read_only && <div className="edu-delegation"><ShieldCheck size={17} />管理员查看中{session.delegated_by_name ? `：${session.delegated_by_name}` : ""}。此页面仅允许复制订阅与节点信息。</div>}
      {notice && <div className="edu-notice" role="status"><Check size={16} />{notice}</div>}
      <section className="edu-heading"><div><h1>账户中心</h1><p>查看订阅信息、流量与可用节点。</p></div><span className="edu-status"><i />正常</span></section>
      <section className="edu-summary" aria-label="账户摘要">
        <Summary icon={<UserRound size={19} />} label="账号" value={role} detail="当前账号" />
        <Summary icon={<Signal size={19} />} label="状态" value="正常" detail="账户状态" success />
        <Summary icon={<Activity size={19} />} label="剩余流量" value={traffic.remaining} detail={traffic.total === "不限" ? "不限额" : `总流量 ${traffic.total}`} />
        <Summary icon={<CalendarDays size={19} />} label="到期时间" value={formatDate(dashboard.expires_at)} detail="订阅有效期" />
      </section>
      <div className="edu-grid">
        <div className="edu-main-column">
          <section className="edu-panel">
            <header><span className="edu-panel__icon"><Link2 size={19} /></span><div><h2>订阅管理</h2><p>复制订阅地址或导入 Clash / Mihomo 客户端。</p></div></header>
            <div className="edu-subscription">
              <label>订阅地址<div className="edu-link-input"><input value={dashboard.subscription_url} readOnly aria-label="订阅地址" /><button className="edu-button edu-button--dark" onClick={() => void copyValue(dashboard.subscription_url).then(() => setNotice("订阅地址已复制")).catch(() => setNotice("复制失败，请手动选择订阅地址"))}><Copy size={16} />复制</button></div></label>
              <div className="edu-subscription__actions">{!session.read_only && <button className="edu-button edu-button--secondary" onClick={() => { window.location.href = `clash://install-config?url=${encodeURIComponent(dashboard.subscription_url)}`; }}><Download size={16} />Clash / Mihomo</button>}{!session.read_only && <button className="edu-text-button edu-text-button--danger" onClick={() => setShowReset(true)}><RefreshCw size={15} />重置订阅地址</button>}</div>
            </div>
          </section>
          <section className="edu-panel">
            <header><span className="edu-panel__icon"><Server size={19} /></span><div><h2>可用节点</h2><p>点击节点即可复制单节点链接，用于直接导入客户端。</p></div><strong className="edu-count">{dashboard.nodes.filter((node) => node.status === "online").length} 在线　{dashboard.nodes.length} 全部</strong></header>
            <div className="edu-node-list">{dashboard.nodes.length === 0 ? <div className="edu-empty">当前套餐没有可用节点。</div> : dashboard.nodes.map((node, index) => <NodeCard key={`${node.name}-${node.entry_name}-${index}`} node={node} onCopied={(message) => setNotice(message)} />)}</div>
          </section>
        </div>
        <aside className="edu-side-column">
          <section className="edu-panel">
            <header><span className="edu-panel__icon"><Activity size={19} /></span><div><h2>流量使用</h2><p>当前套餐流量使用情况。</p></div></header>
            <div className="edu-traffic"><strong>{traffic.percent}</strong><span>已使用</span><div className="edu-progress"><i style={{ width: `${traffic.progress}%` }} /></div><dl><div><dt>已用</dt><dd>{formatBytes(dashboard.traffic_used_bytes)}</dd></div><div><dt>总量</dt><dd>{traffic.total}</dd></div></dl>{dashboard.traffic_reset_at && <small>下次重置：{formatDate(dashboard.traffic_reset_at)}</small>}</div>
          </section>
          <section className="edu-panel">
            <header><span className="edu-panel__icon"><CircleUserRound size={19} /></span><div><h2>账户安全</h2><p>账户信息与密码。</p></div></header>
            <dl className="edu-account-data"><div><dt>邮箱</dt><dd>{dashboard.email ?? "未设置"}</dd></div><div><dt>设备限制</dt><dd>{dashboard.device_limit ? `${dashboard.device_limit} 台` : "不限制"}</dd></div><div><dt>速率限制</dt><dd>{dashboard.speed_limit_mbps ? `${dashboard.speed_limit_mbps} Mbps` : "不限制"}</dd></div></dl>
            {session.read_only ? <div className="edu-readonly">管理员查看模式下不允许修改账户。</div> : <PasswordForm onSaved={() => setNotice("密码已更新，其他登录会话已退出")} />}
          </section>
        </aside>
      </div>
    </div>
    {showReset && <PasswordDialog title="重置订阅地址" description="重置后，旧订阅地址会立即失效。单节点链接不会改变。" action="确认重置" onClose={() => setShowReset(false)} onSubmit={async (currentPassword) => { const result = await portalApi.rotateSubscription(currentPassword); setDashboard((current) => current ? { ...current, subscription_url: result.url } : current); setNotice("订阅地址已重置，旧地址已失效"); setShowReset(false); }} />}
  </main>;
}

function Summary({ icon, label, value, detail, success = false }: { icon: ReactNode; label: string; value: string; detail: string; success?: boolean }) { return <div className="edu-summary__item"><span className="edu-summary__icon">{icon}</span><div><strong>{value}{success && <i className="edu-inline-dot" />}</strong><span>{label}</span><small>{detail}</small></div></div>; }

function NodeCard({ node, onCopied }: { node: PortalDashboard["nodes"][number]; onCopied: (message: string) => void }) {
  const uri = node.uri;
  const unavailable = !uri;
  return <button type="button" className={`edu-node ${unavailable ? "is-unavailable" : ""}`} disabled={unavailable} onClick={() => { if (uri) void copyValue(uri).then(() => onCopied(`${node.name} 单节点链接已复制`)).catch(() => onCopied("复制失败，请重试")); }}><span className="edu-node__icon"><Network size={18} /></span><span className="edu-node__copy"><strong>{node.name}</strong><small>{node.entry_name || node.protocol.toUpperCase()}</small></span><span className={`edu-node__status ${node.status === "online" ? "is-online" : ""}`}>{unavailable ? node.error : node.status === "online" ? "在线" : "离线"}</span><Clipboard size={16} /></button>;
}

function PasswordForm({ onSaved }: { onSaved: () => void }) {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError("");
    try {
      await portalApi.changePassword(currentPassword, newPassword, newPassword);
      setCurrentPassword("");
      setNewPassword("");
      onSaved();
    } catch (reason) {
      setError(reason instanceof ApiError ? reason.message : "密码更新失败，请稍后重试");
    } finally {
      setLoading(false);
    }
  }

  return <form className="edu-password-form" onSubmit={submit}>
    {error && <div className="edu-alert" role="alert"><TriangleAlert size={16} />{error}</div>}
    <label>当前密码<input type="password" autoComplete="current-password" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} required /></label>
    <label>新密码<input type="password" autoComplete="new-password" minLength={8} value={newPassword} onChange={(event) => setNewPassword(event.target.value)} required /></label>
    <button className="edu-button edu-button--dark" type="submit" disabled={loading}><KeyRound size={16} />{loading ? "更新中…" : "更新密码"}</button>
  </form>;
}

function PasswordDialog({ title, description, action, showNewPassword = false, onClose, onSubmit }: { title: string; description: string; action: string; showNewPassword?: boolean; onClose: () => void; onSubmit: (currentPassword: string, newPassword?: string, confirmPassword?: string) => Promise<void> }) {
  const [currentPassword, setCurrentPassword] = useState(""); const [newPassword, setNewPassword] = useState(""); const [confirmPassword, setConfirmPassword] = useState(""); const [loading, setLoading] = useState(false); const [error, setError] = useState("");
  async function submit(event: FormEvent) { event.preventDefault(); setLoading(true); setError(""); try { await onSubmit(currentPassword, newPassword, confirmPassword); } catch (reason) { setError(reason instanceof ApiError ? reason.message : "操作失败，请重试"); } finally { setLoading(false); } }
  return <div className="edu-dialog-backdrop" role="presentation"><section className="edu-dialog" role="dialog" aria-modal="true" aria-labelledby="edu-dialog-title"><header><h2 id="edu-dialog-title">{title}</h2><p>{description}</p></header>{error && <div className="edu-alert" role="alert"><TriangleAlert size={17} />{error}</div>}<form onSubmit={submit}><label>当前密码<input type="password" autoComplete="current-password" value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} required autoFocus /></label>{showNewPassword && <><label>新密码<input type="password" autoComplete="new-password" minLength={8} value={newPassword} onChange={(event) => setNewPassword(event.target.value)} required /></label><label>确认新密码<input type="password" autoComplete="new-password" minLength={8} value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} required /></label></>}<footer><button type="button" className="edu-button edu-button--secondary" disabled={loading} onClick={onClose}>取消</button><button type="submit" className="edu-button edu-button--primary" disabled={loading}>{loading ? "处理中…" : action}</button></footer></form></section></div>;
}

function trafficSummary(used: number, limit: number) { if (!limit) return { total: "不限", remaining: "不限", percent: "不限", progress: 0 }; const remaining = Math.max(limit - used, 0); const progress = Math.min(100, Math.round((used / limit) * 100)); return { total: formatBytes(limit), remaining: formatBytes(remaining), percent: `${progress}%`, progress }; }
function formatBytes(value: number) { if (!value) return "0 B"; const units = ["B", "KB", "MB", "GB", "TB", "PB"]; const exponent = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1); const amount = value / 1024 ** exponent; return `${amount >= 100 || exponent === 0 ? amount.toFixed(0) : amount.toFixed(2)} ${units[exponent]}`; }
function formatDate(value: string | null) { if (!value) return "长期"; return new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit" }).format(new Date(value)); }
async function copyValue(value: string) { if (navigator.clipboard?.writeText) return navigator.clipboard.writeText(value); const input = document.createElement("textarea"); input.value = value; input.style.position = "fixed"; input.style.opacity = "0"; document.body.append(input); input.select(); const copied = document.execCommand("copy"); input.remove(); if (!copied) throw new Error("clipboard unavailable"); }
