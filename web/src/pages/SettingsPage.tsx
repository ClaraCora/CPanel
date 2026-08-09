import { useEffect, useMemo, useState, type FormEvent } from "react";
import { Eye, EyeOff, KeyRound, LockKeyhole, RefreshCw, Save, ScrollText, Send, ShieldCheck, UserRound } from "lucide-react";
import { ApiError, api } from "../api";
import { Button, EmptyState, Field, PageHeader, TableSkeleton, formatPreciseDate, useToast } from "../components/ui";
import { useResource } from "../hooks";
import type { Admin, Setting, SubscriptionAccessEvent } from "../types";

type SettingField = { key: string; label: string; type?: "text" | "number" | "select" | "password" | "textarea" | "toggle" | "time"; helper?: string; sensitive?: boolean; generate?: boolean; options?: { value: string; label: string }[]; defaultValue?: string; min?: number; max?: number };
type SettingSection = { id: string; label: string; description: string; fields?: SettingField[] };

const subscriptionSection: SettingSection = { id: "subscription", label: "订阅设置", description: "Clash Meta 输出、浏览器拦截和拉取记录", fields: [{ key: "base_url", label: "订阅基础地址" }, { key: "cache_seconds", label: "缓存时间（秒）", type: "number", defaultValue: "60" }, { key: "format", label: "输出格式", type: "select", options: [{ value: "clash-meta", label: "Clash Meta" }], defaultValue: "clash-meta" }, { key: "block_browser_access", label: "拦截常规浏览器", type: "toggle", defaultValue: "false" }, { key: "ua_whitelist", label: "UA 白名单", type: "textarea", helper: "每行一个 UA 关键字，不区分大小写" }] };

const sections: SettingSection[] = [
  { id: "account", label: "管理员账户", description: "登录资料、邮箱和密码" },
  { id: "site", label: "站点", description: "平台名称、外部地址、语言和时区", fields: [{ key: "platform_name", label: "平台名称", defaultValue: "CPanel" }, { key: "site_url", label: "站点 URL", helper: "用于生成 Agent 和订阅地址" }, { key: "timezone", label: "时区", defaultValue: "Asia/Shanghai" }, { key: "default_language", label: "默认语言", type: "select", options: [{ value: "zh-CN", label: "简体中文" }, { value: "en", label: "English" }], defaultValue: "zh-CN" }, { key: "footer_text", label: "页脚文本", type: "textarea" }] },
  { id: "agent", label: "Agent 接入", description: "Corade 通讯密钥、心跳与同步参数", fields: [{ key: "external_url", label: "外部控制地址", defaultValue: window.location.origin }, { key: "installer_url", label: "安装脚本地址", defaultValue: "https://raw.githubusercontent.com/ClaraCora/CPanelde/main/install.sh" }, { key: "communication_key", label: "统一通讯密钥", type: "password", sensitive: true, generate: true, helper: "仅用于 Agent 首次登记或身份重置；至少 32 个字符，保存后不再显示明文" }, { key: "heartbeat_seconds", label: "心跳间隔（秒）", type: "number", defaultValue: "60" }, { key: "offline_threshold_seconds", label: "离线阈值（秒）", type: "number", defaultValue: "180" }, { key: "fallback_pull_seconds", label: "REST 兜底间隔（秒）", type: "number", defaultValue: "60" }, { key: "max_message_bytes", label: "最大消息大小（字节）", type: "number", defaultValue: "2097152" }, { key: "allow_legacy_protocol", label: "允许旧通讯", type: "toggle", defaultValue: "true", helper: "关闭前必须所有未归档服务器都显示 V2 加密；关闭后旧 Agent 将返回 404" }] },
  { id: "security", label: "安全", description: "管理员会话和登录保护", fields: [{ key: "session_ttl_minutes", label: "会话有效期（分钟）", type: "number", defaultValue: "720" }, { key: "password_min_length", label: "密码最小长度", type: "number", defaultValue: "8", min: 8, max: 128 }, { key: "max_login_failures", label: "登录失败锁定次数", type: "number", defaultValue: "8" }, { key: "trusted_proxy_cidrs", label: "可信代理 CIDR", type: "textarea", helper: "每行一个 CIDR" }] },
  { id: "node_defaults", label: "节点默认值", description: "新建节点时使用的内核、监听与上报参数", fields: [{ key: "default_kernel", label: "默认内核", type: "select", options: [{ value: "xray", label: "Xray" }, { value: "singbox", label: "sing-box" }], defaultValue: "xray" }, { key: "listen_ip", label: "默认监听地址", defaultValue: "0.0.0.0" }, { key: "telemetry_seconds", label: "遥测上报间隔（秒）", type: "number", defaultValue: "60" }, { key: "certificate_mode", label: "证书模式", type: "select", options: [{ value: "manual", label: "手动配置" }, { value: "acme", label: "ACME 自动申请" }], defaultValue: "manual" }] },
  { id: "certificate", label: "证书与 DNS", description: "ACME 账号和 DNS Provider 凭据", fields: [{ key: "acme_email", label: "ACME 邮箱" }, { key: "dns_provider", label: "DNS Provider" }, { key: "dns_api_token", label: "DNS API Token", type: "password", sensitive: true, helper: "保存后不再显示明文" }, { key: "http01_port", label: "HTTP-01 端口", type: "number", defaultValue: "80" }] },
  { id: "tgbot", label: "TG Bot", description: "管理员查询与昨日流量排行推送", fields: [{ key: "enabled", label: "启用 TG Bot", type: "toggle", defaultValue: "false", helper: "启用后通过长轮询接收命令，不需要配置 Webhook" }, { key: "bot_token", label: "BotFather 密钥", type: "password", sensitive: true, helper: "填写 @BotFather 提供的完整密钥，保存后不再显示明文" }, { key: "admin_telegram_id", label: "管理员 Telegram ID", helper: "先私聊 Bot 发送 /id，再将返回的数字填写到这里" }, { key: "daily_report_enabled", label: "推送昨日排行榜", type: "toggle", defaultValue: "true" }, { key: "daily_report_time", label: "日报推送时间", type: "time", defaultValue: "09:00", helper: "使用站点设置中的时区；内容包含昨日上传、下载及节点和用户 Top 5" }] },
  { id: "retention", label: "数据保留", description: "在线设备、流量和审计日志保留时间；服务器与节点指标仅保留最新值", fields: [{ key: "devices_days", label: "在线设备保留天数", type: "number", defaultValue: "30" }, { key: "traffic_days", label: "流量明细保留天数", type: "number", defaultValue: "90" }, { key: "audit_days", label: "审计日志保留天数", type: "number", defaultValue: "180" }, { key: "subscription_access_days", label: "订阅拉取记录保留天数", type: "number", defaultValue: "30" }] },
];

export function SettingsPage({ admin, onAdminChange }: { admin: Admin; onAdminChange: (admin: Admin) => void }) {
  const [active, setActive] = useState("account");
  const section = useMemo(() => sections.find((item) => item.id === active)!, [active]);
  return <div className="page"><PageHeader title="系统设置" description="维护管理员账户、站点、Agent、安全和数据参数" /><div className="settings-layout"><nav className="settings-nav" aria-label="设置分区">{sections.map((item) => <button type="button" className={item.id === active ? "active" : ""} onClick={() => setActive(item.id)} key={item.id}><strong>{item.label}</strong><span>{item.description}</span></button>)}</nav><section className="settings-content"><header><div><h2>{section.label}</h2><p>{section.description}</p></div>{section.fields?.some((field) => field.sensitive) && <span className="security-note"><KeyRound size={15} />敏感值加密存储</span>}</header>{active === "account" ? <AccountSettings admin={admin} onAdminChange={onAdminChange} /> : <SystemSettingForm section={section} />}</section></div></div>;
}

export function SubscriptionPage() {
  return <div className="page"><PageHeader title="订阅" description="管理订阅输出、访问策略和拉取记录" /><div className="settings-layout settings-layout--standalone"><section className="settings-content"><header><div><h2>{subscriptionSection.label}</h2><p>{subscriptionSection.description}</p></div></header><SystemSettingForm section={subscriptionSection} /><SubscriptionAccessLog /></section></div></div>;
}

function AccountSettings({ admin, onAdminChange }: { admin: Admin; onAdminChange: (admin: Admin) => void }) {
  const { data: securitySettings } = useResource<Setting[]>("/settings/security", []);
  const [profile, setProfile] = useState({ name: admin.name, email: admin.email });
  const [password, setPassword] = useState({ current_password: "", new_password: "", confirm_password: "" });
  const [profileErrors, setProfileErrors] = useState<Record<string, string>>({});
  const [passwordErrors, setPasswordErrors] = useState<Record<string, string>>({});
  const [profileSaving, setProfileSaving] = useState(false);
  const [passwordSaving, setPasswordSaving] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const toast = useToast();
  const configuredMinimum = Number(securitySettings.find((item) => item.key === "password_min_length")?.value ?? 8);
  const passwordMinimum = Number.isInteger(configuredMinimum) && configuredMinimum >= 8 ? configuredMinimum : 8;
  useEffect(() => setProfile({ name: admin.name, email: admin.email }), [admin.email, admin.name]);

  async function saveProfile(event: FormEvent) {
    event.preventDefault();
    const errors: Record<string, string> = {};
    if (!profile.name.trim()) errors.name = "请填写管理员名称";
    if (!/^\S+@\S+\.\S+$/.test(profile.email.trim())) errors.email = "请填写有效的管理员邮箱";
    setProfileErrors(errors);
    if (Object.keys(errors).length) return;
    setProfileSaving(true);
    try {
      const updated = await api.patch<Admin>("/account/profile", { name: profile.name.trim(), email: profile.email.trim() });
      onAdminChange(updated);
      toast("管理员资料已保存");
    } catch (reason) {
      if (reason instanceof ApiError) setProfileErrors(reason.fields);
      toast(reason instanceof ApiError ? reason.message : "管理员资料保存失败", "error");
    } finally { setProfileSaving(false); }
  }

  async function savePassword(event: FormEvent) {
    event.preventDefault();
    const errors: Record<string, string> = {};
    if (!password.current_password) errors.current_password = "请输入当前密码";
    if (Array.from(password.new_password).length < passwordMinimum) errors.new_password = `新密码至少需要 ${passwordMinimum} 个字符`;
    if (password.new_password !== password.confirm_password) errors.confirm_password = "两次输入的新密码不一致";
    setPasswordErrors(errors);
    if (Object.keys(errors).length) return;
    setPasswordSaving(true);
    try {
      await api.patch("/account/password", password);
      setPassword({ current_password: "", new_password: "", confirm_password: "" });
      toast("密码已更新，其他管理会话已退出");
    } catch (reason) {
      if (reason instanceof ApiError) setPasswordErrors(reason.fields);
      toast(reason instanceof ApiError ? reason.message : "密码修改失败", "error");
    } finally { setPasswordSaving(false); }
  }

  const passwordType = showPassword ? "text" : "password";
  return <div className="account-settings"><form className="settings-form account-form" onSubmit={saveProfile}><div className="account-form__heading"><UserRound size={18} /><div><h3>账户资料</h3><p>用于登录后台、侧栏展示和审计记录。</p></div></div><Field label="管理员名称" required error={profileErrors.name}><input value={profile.name} onChange={(event) => setProfile((current) => ({ ...current, name: event.target.value }))} autoComplete="name" /></Field><Field label="登录邮箱" required error={profileErrors.email}><input type="email" value={profile.email} onChange={(event) => setProfile((current) => ({ ...current, email: event.target.value }))} autoComplete="username" /></Field><footer><span><ShieldCheck size={15} />修改会写入管理员审计日志</span><Button type="submit" variant="primary" loading={profileSaving}><Save size={16} />保存账户资料</Button></footer></form><form className="settings-form account-form account-form--password" onSubmit={savePassword}><div className="account-form__heading"><LockKeyhole size={18} /><div><h3>修改密码</h3><p>修改前必须验证当前密码，新密码至少 {passwordMinimum} 个字符。</p></div><button type="button" className="icon-button account-password-toggle" aria-label={showPassword ? "隐藏密码" : "显示密码"} title={showPassword ? "隐藏密码" : "显示密码"} onClick={() => setShowPassword((value) => !value)}>{showPassword ? <EyeOff size={17} /> : <Eye size={17} />}</button></div><Field label="当前密码" required error={passwordErrors.current_password}><input type={passwordType} autoComplete="current-password" value={password.current_password} onChange={(event) => setPassword((current) => ({ ...current, current_password: event.target.value }))} /></Field><Field label="新密码" required error={passwordErrors.new_password}><input type={passwordType} minLength={passwordMinimum} autoComplete="new-password" value={password.new_password} onChange={(event) => setPassword((current) => ({ ...current, new_password: event.target.value }))} /></Field><Field label="确认新密码" required error={passwordErrors.confirm_password}><input type={passwordType} minLength={passwordMinimum} autoComplete="new-password" value={password.confirm_password} onChange={(event) => setPassword((current) => ({ ...current, confirm_password: event.target.value }))} /></Field><footer><span><ShieldCheck size={15} />保存后当前设备保持登录，其他会话将退出</span><Button type="submit" variant="primary" loading={passwordSaving}><KeyRound size={16} />更新密码</Button></footer></form></div>;
}

function SystemSettingForm({ section }: { section: SettingSection }) {
  const fields = section.fields ?? [];
  const { data, loading, error, reload } = useResource<Setting[]>(`/settings/${section.id}`, []);
  const [values, setValues] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const toast = useToast();
  useEffect(() => { const stored = new Map(data.map((item) => [item.key, item.value])); setValues(Object.fromEntries(fields.map((field) => [field.key, field.sensitive && stored.has(field.key) ? "" : stored.has(field.key) ? String(stored.get(field.key) ?? "") : field.defaultValue ?? ""]))); }, [data, fields]);

  function generateKey(field: SettingField) {
    const bytes = crypto.getRandomValues(new Uint8Array(32));
    const value = btoa(String.fromCharCode(...bytes)).replaceAll("+", "-").replaceAll("/", "_").replaceAll("=", "");
    setValues((current) => ({ ...current, [field.key]: `cpa_${value}` }));
  }

  async function save(event: FormEvent) {
    event.preventDefault(); setSaving(true);
    const payload: Record<string, unknown> = {};
    fields.forEach((field) => { const raw = values[field.key] ?? ""; if (field.sensitive && !raw) return; payload[field.key] = field.type === "number" ? Number(raw || 0) : field.type === "toggle" ? raw === "true" : raw; });
    try { await api.patch(`/settings/${section.id}`, { values: payload, sensitive_keys: fields.filter((field) => field.sensitive && values[field.key]).map((field) => field.key) }); toast(`${section.label}设置已保存`); await reload(); }
    catch (reason) { toast(reason instanceof ApiError ? reason.message : "设置保存失败", "error"); }
    finally { setSaving(false); }
  }

  async function sendTestMessage() {
    setTesting(true);
    try {
      await api.post("/settings/tgbot/test");
      toast("测试消息已发送到绑定的 Telegram 账号");
    } catch (reason) {
      toast(reason instanceof ApiError ? reason.message : "测试消息发送失败", "error");
    } finally { setTesting(false); }
  }

  if (loading) return <div className="settings-loading"><TableSkeleton columns={2} rows={5} /></div>;
  if (error) return <EmptyState title="设置加载失败" description={error} action={<Button onClick={() => void reload()}>重新加载</Button>} />;
  return <form onSubmit={save} className="settings-form">
    {fields.map((field) => <Field key={field.key} label={field.label} helper={field.helper}>{field.type === "toggle" ? <label className="settings-toggle"><input type="checkbox" checked={(values[field.key] ?? "true") === "true"} onChange={(event) => setValues((current) => ({ ...current, [field.key]: String(event.target.checked) }))} /><span>{(values[field.key] ?? "true") === "true" ? "启用" : "关闭"}</span></label> : field.type === "textarea" ? <textarea rows={4} value={values[field.key] ?? ""} onChange={(event) => setValues((current) => ({ ...current, [field.key]: event.target.value }))} /> : field.type === "select" ? <select value={values[field.key] ?? ""} onChange={(event) => setValues((current) => ({ ...current, [field.key]: event.target.value }))}>{field.options?.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select> : field.generate ? <div className="input-with-action"><input type={field.type ?? "text"} min={field.min} max={field.max} value={values[field.key] ?? ""} autoComplete="new-password" placeholder="留空表示不修改" onChange={(event) => setValues((current) => ({ ...current, [field.key]: event.target.value }))} /><Button type="button" onClick={() => generateKey(field)}><RefreshCw size={15} />生成</Button></div> : <input type={field.type ?? "text"} min={field.min} max={field.max} value={values[field.key] ?? ""} autoComplete={field.sensitive ? "new-password" : undefined} placeholder={field.sensitive ? "留空表示不修改" : undefined} onChange={(event) => setValues((current) => ({ ...current, [field.key]: event.target.value }))} />}</Field>)}
    <footer><span><ShieldCheck size={15} />修改将写入管理员审计日志</span><div className="settings-form__actions">{section.id === "tgbot" && <Button type="button" loading={testing} disabled={saving} onClick={() => void sendTestMessage()}><Send size={16} />发送测试消息</Button>}<Button type="submit" variant="primary" loading={saving} disabled={testing}><Save size={16} />保存设置</Button></div></footer>
  </form>;
}

const accessOutcome = {
  allowed: { label: "已放行", tone: "success" },
  blocked: { label: "已拦截", tone: "warning" },
  not_found: { label: "无效令牌", tone: "neutral" },
  failed: { label: "拉取失败", tone: "danger" },
} as const;

function SubscriptionAccessLog() {
  const { data, loading, error, reload } = useResource<SubscriptionAccessEvent[]>("/settings/subscription/access-log", []);
  return <section className="subscription-access" aria-labelledby="subscription-access-title"><header><div><ScrollText size={18} /><div><h3 id="subscription-access-title">订阅拉取记录</h3><p>最近 200 条</p></div></div><Button type="button" onClick={() => void reload()} disabled={loading}><RefreshCw size={15} />刷新</Button></header>{loading ? <div className="subscription-access__loading"><TableSkeleton columns={5} rows={5} /></div> : error ? <EmptyState title="拉取记录加载失败" description={error} action={<Button type="button" onClick={() => void reload()}>重新加载</Button>} /> : data.length === 0 ? <EmptyState title="暂无拉取记录" description="新的订阅请求会显示在这里。" /> : <div className="table-scroll subscription-access__table"><table><thead><tr><th>账号</th><th>结果</th><th>IP 地址</th><th>User-Agent</th><th>拉取时间</th></tr></thead><tbody>{data.map((item) => { const outcome = accessOutcome[item.outcome] ?? accessOutcome.failed; return <tr key={item.id}><td><strong className="cell-primary">{item.user_name || "未知账号"}</strong><span className="resource-id">{item.user_id || "未匹配账号"}</span></td><td><span className={`status status--${outcome.tone}`}><span className="status__dot" aria-hidden="true" />{outcome.label}</span><span className="subscription-access__code">HTTP {item.status_code}</span></td><td className="mono">{item.ip_address || "未知"}</td><td><code title={item.user_agent}>{item.user_agent || "未提供"}</code></td><td className="mono">{formatPreciseDate(item.created_at)}</td></tr>; })}</tbody></table></div>}</section>;
}
