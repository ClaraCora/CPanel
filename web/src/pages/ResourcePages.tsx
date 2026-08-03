import { useMemo, useState, type FormEvent, type ReactNode } from "react";
import { Copy, Edit3, Link2, Plus, Power, RefreshCw, Search, Terminal, Trash2, UserPlus, UsersRound } from "lucide-react";
import { ApiError, api } from "../api";
import { Button, ConfirmDialog, Drawer, EmptyState, Field, PageHeader, RowMenu, StatusBadge, TableSkeleton, formatBytes, formatDate, formatDateWithYear, formatPreciseDate, useToast } from "../components/ui";
import { NodeMultiSelect, type MultiSelectOption } from "../components/NodeMultiSelect";
import { OutboundConfigForm, defaultOutboundSettings, parseOutboundSettings, validateOutboundSettingsValue } from "../components/OutboundConfigForm";
import { RouteRulesEditor, validateRouteRulesValue } from "../components/RouteRulesEditor";
import { useResource } from "../hooks";
import type { AccessGroup, Machine, Node, Outbound, Plan, RoutePolicy, User } from "../types";

type Resource = { id: string; status: string; created_at: string; updated_at: string };
type Column<T> = { label: string; render: (item: T) => ReactNode; className?: string };
type FieldSpec = {
  key: string;
  label: string;
  type?: "text" | "number" | "textarea" | "select" | "node-picker" | "route-rules" | "outbound-config" | "json" | "email" | "datetime-local";
  required?: boolean;
  helper?: string;
  options?: (MultiSelectOption & { exclusive?: boolean })[];
	routeOutbounds?: Outbound[];
  placeholder?: string;
  editOnly?: boolean;
  lockedValue?: string;
};

type ResourcePageProps<T extends Resource> = {
  title: string;
  description: string;
  endpoint: string;
  createLabel: string;
  columns: Column<T>[];
  fields: FieldSpec[];
  defaults: Record<string, string>;
  transform: (values: Record<string, string>, editing: boolean) => unknown;
  toValues: (item: T) => Record<string, string>;
  enabledStatus?: string;
  disabledStatus?: string;
  installable?: boolean;
  upgradeable?: boolean;
  subscriptionActions?: boolean;
  quickAccountCreation?: boolean;
  deletable?: boolean;
  identifierAction?: { label: string; value: (item: T) => string };
  protectedItem?: (item: T) => boolean;
  refreshIntervalMs?: number;
};

function ResourcePage<T extends Resource>({ title, description, endpoint, createLabel, columns, fields, defaults, transform, toValues, enabledStatus = "active", disabledStatus = "disabled", installable = false, upgradeable = false, subscriptionActions = false, quickAccountCreation = false, deletable = false, identifierAction, protectedItem, refreshIntervalMs = 0 }: ResourcePageProps<T>) {
	deletable = deletable || ["/machines", "/access-groups", "/plans", "/route-policies"].includes(endpoint);
	if (endpoint === "/route-policies") enabledStatus = "published";
  const { data, loading, error, reload } = useResource<T[]>(endpoint, [], refreshIntervalMs);
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<T | null>(null);
  const [values, setValues] = useState(defaults);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [changingStatus, setChangingStatus] = useState("");
  const [copying, setCopying] = useState("");
  const [copyingIdentifier, setCopyingIdentifier] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<T | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [install, setInstall] = useState<{ name: string; command: string } | null>(null);
  const [upgradeTarget, setUpgradeTarget] = useState<T | null>(null);
  const [upgrading, setUpgrading] = useState(false);
  const [formError, setFormError] = useState("");
  const [quickCreating, setQuickCreating] = useState<"user" | "friend" | "">("");
  const toast = useToast();
  const filtered = useMemo(() => data.filter((item) => !query.trim() || JSON.stringify(item).toLowerCase().includes(query.trim().toLowerCase())), [data, query]);

  function showCreate() {
    setEditing(null);
    setValues({
      ...defaults,
      ...(endpoint === "/plans" ? { speed_limit_mbps: "0" } : {}),
    });
    setErrors({});
    setFormError("");
    setOpen(true);
  }

  function showEdit(item: T) {
    setEditing(item);
    setValues(toValues(item));
    setErrors({});
    setFormError("");
    setOpen(true);
  }

  async function save() {
    if (saving) return;
    const next: Record<string, string> = {};
    fields.forEach((field) => {
      if (field.required && !values[field.key]?.trim()) next[field.key] = `请填写${field.label}`;
      if (field.type === "email" && values[field.key]?.trim() && !/^\S+@\S+\.\S+$/.test(values[field.key].trim())) next[field.key] = "请填写有效的邮箱地址";
      if (field.type === "json" && values[field.key]) {
        try { JSON.parse(values[field.key]); } catch { next[field.key] = `${field.label}必须是有效 JSON`; }
      }
		if (field.type === "route-rules") {
			const message = validateRouteRulesValue(values[field.key] ?? "[]");
			if (message) next[field.key] = message;
		}
		if (field.type === "outbound-config") {
			const message = validateOutboundSettingsValue(values.protocol ?? "", values.settings ?? "{}");
			if (message) next[field.key] = message;
		}
    });
    setErrors(next);
    setFormError("");
    if (Object.keys(next).length) return;
    setSaving(true);
    try {
      if (editing) await api.patch(`${endpoint}/${editing.id}`, transform(values, true));
      else await api.post(endpoint, transform(values, false));
      toast(editing ? `${title}已更新` : `${createLabel}已创建`);
      setOpen(false);
      await reload();
    } catch (reason) {
      if (reason instanceof ApiError) {
        setErrors(reason.fields);
        setFormError(reason.message);
        toast(reason.message, "error");
      } else {
        setFormError(reason instanceof Error ? reason.message : "保存失败，请重试");
        toast("保存失败，请重试", "error");
      }
    } finally {
      setSaving(false);
    }
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    void save();
  }

  async function toggleStatus(item: T) {
    const nextStatus = item.status === disabledStatus ? enabledStatus : disabledStatus;
    setChangingStatus(item.id);
    try {
      await api.patch(`${endpoint}/${item.id}`, { status: nextStatus });
      toast(nextStatus === disabledStatus ? `${title}已${disabledStatus === "paused" ? "暂停" : "停用"}` : `${title}已恢复`);
      await reload();
    } catch (reason) {
      toast(reason instanceof ApiError ? reason.message : "状态修改失败", "error");
    } finally {
      setChangingStatus("");
    }
  }

  async function copySubscription(item: T) {
    setCopying(item.id);
    try {
      const result = await api.get<{ url: string }>(`${endpoint}/${item.id}/subscription`);
      await writeClipboard(result.url);
      toast("订阅链接已复制");
    } catch (reason) {
      toast(reason instanceof ApiError ? reason.message : "订阅链接复制失败，请重试", "error");
    } finally {
      setCopying("");
    }
  }

  async function copyIdentifier(item: T) {
    if (!identifierAction) return;
    setCopyingIdentifier(item.id);
    try {
      await writeClipboard(identifierAction.value(item));
      toast(`${identifierAction.label} 已复制`);
    } catch {
      toast(`${identifierAction.label} 复制失败，请重试`, "error");
    } finally {
      setCopyingIdentifier("");
    }
  }

  async function deleteResource() {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await api.delete(`${endpoint}/${deleteTarget.id}`);
      toast(subscriptionActions ? "账号已删除，原订阅链接已失效" : `${resourceName(deleteTarget)} 已删除`);
      setDeleteTarget(null);
      await reload();
    } catch (reason) {
      toast(reason instanceof ApiError ? reason.message : `${title}删除失败，请重试`, "error");
    } finally {
      setDeleting(false);
    }
  }

  async function loadInstallation(item: T) {
    try {
      const result = await api.get<{ command: string }>(`${endpoint}/${item.id}/installation`);
      setInstall({ name: "name" in item ? String(item.name) : item.id, command: result.command });
    } catch (reason) {
      toast(reason instanceof ApiError ? reason.message : "安装命令生成失败", "error");
    }
  }

  async function requestUpgrade() {
    if (!upgradeTarget) return;
    setUpgrading(true);
    try {
      await api.post(`${endpoint}/${upgradeTarget.id}/agent-upgrade`);
      toast("升级任务已下发，Agent 将在下一次心跳时执行");
      setUpgradeTarget(null);
      await reload();
    } catch (reason) {
      const message = reason instanceof ApiError ? reason.message : "升级任务下发失败，请重试";
      toast(message, "error");
    } finally {
      setUpgrading(false);
    }
  }

  async function quickCreateAccount(role: "user" | "friend") {
    if (quickCreating) return;
    setQuickCreating(role);
    try {
      await api.post(`${endpoint}/quick/${role}`);
      toast(`${role === "friend" ? "朋友" : "用户"}已添加，可在列表中编辑资料`);
      await reload();
    } catch (reason) {
      toast(reason instanceof ApiError ? reason.message : "一键添加失败，请重试", "error");
    } finally {
      setQuickCreating("");
    }
  }

  function updateValue(key: string, value: string) {
    setValues((current) => ({ ...current, [key]: value }));
    setErrors((current) => ({ ...current, [key]: "" }));
  }

  return <div className="page">
    <PageHeader title={title} description={description} actions={<>{quickAccountCreation && <><Button type="button" loading={quickCreating === "user"} disabled={Boolean(quickCreating)} onClick={() => void quickCreateAccount("user")}><UserPlus size={16} />{quickCreating === "user" ? "添加中…" : "一键添加用户"}</Button><Button type="button" loading={quickCreating === "friend"} disabled={Boolean(quickCreating)} onClick={() => void quickCreateAccount("friend")}><UsersRound size={16} />{quickCreating === "friend" ? "添加中…" : "一键添加朋友"}</Button></>}<Button type="button" variant="primary" disabled={Boolean(quickCreating)} onClick={showCreate}><Plus size={16} />{createLabel}</Button></>} />
    <div className="toolbar"><label className="search-box"><Search size={16} /><span className="sr-only">搜索{title}</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={`搜索${title}`} /></label><span className="toolbar__count">{filtered.length} 条记录</span></div>
    <div className="table-surface">
      {loading ? <TableSkeleton columns={columns.length + 2} /> : error ? <EmptyState title={`${title}加载失败`} description={error} action={<Button onClick={() => void reload()}>重新加载</Button>} /> : filtered.length === 0 ? <EmptyState title={data.length ? "没有符合搜索条件的记录" : `还没有${title}`} description={data.length ? "更换搜索词后重试。" : `创建第一条${title}记录开始管理。`} action={!data.length && <Button variant="primary" onClick={showCreate}><Plus size={16} />{createLabel}</Button>} /> : (
        <div className="table-scroll"><table><thead><tr>{columns.map((column) => <th className={column.className} key={column.label}>{column.label}</th>)}<th>状态</th><th className="col-actions">操作</th></tr></thead><tbody>{filtered.map((item) => {
          const canCopy = !subscriptionActions || subscriptionAvailable(item);
          const protectedRow = protectedItem?.(item) ?? false;
          return <tr key={item.id}>{columns.map((column) => <td className={column.className} key={column.label}>{column.render(item)}</td>)}<td><StatusBadge status={item.status} /></td><td className="row-actions">
            {identifierAction && <button type="button" className="icon-button" aria-label={`复制 ${item.id} 的 ${identifierAction.label}`} title={`复制 ${identifierAction.label}`} disabled={copyingIdentifier === item.id} onClick={() => void copyIdentifier(item)}><Copy size={16} /></button>}
            {subscriptionActions && <button type="button" className="icon-button" aria-label={`复制 ${item.id} 的订阅链接`} title={canCopy ? "复制订阅链接" : "完整令牌尚未保存"} disabled={!canCopy || copying === item.id} onClick={() => void copySubscription(item)}><Link2 size={16} /></button>}
            {upgradeable && <button type="button" className={`icon-button ${agentUpgradeState(item) === "available" ? "icon-button--update" : ""}`} aria-label={`升级 ${resourceName(item)} 的 Agent`} title={upgradeStateLabel(item)} disabled={!canRequestUpgrade(item) || upgrading} onClick={() => setUpgradeTarget(item)}><RefreshCw size={16} /></button>}
            <button type="button" className="icon-button" aria-label={`编辑 ${item.id}`} title="编辑" onClick={() => showEdit(item)}><Edit3 size={16} /></button>
            {deletable && !protectedRow && <button type="button" className="icon-button icon-button--danger" aria-label={`删除 ${resourceName(item)}`} title="删除" onClick={() => setDeleteTarget(item)}><Trash2 size={16} /></button>}
            <RowMenu label={`${item.id} 更多操作`}><button type="button" onClick={() => showEdit(item)}><Edit3 size={15} />编辑</button>{identifierAction && <button type="button" disabled={copyingIdentifier === item.id} onClick={() => void copyIdentifier(item)}><Copy size={15} />复制 {identifierAction.label}</button>}{subscriptionActions && <button type="button" disabled={!canCopy || copying === item.id} onClick={() => void copySubscription(item)}><Link2 size={15} />{canCopy ? "复制订阅链接" : "完整令牌未保存"}</button>}{installable && <button type="button" onClick={() => void loadInstallation(item)}><Terminal size={15} />一键安装 Agent</button>}{!protectedRow && <button type="button" disabled={changingStatus === item.id} onClick={() => void toggleStatus(item)}><Power size={15} />{item.status === disabledStatus ? "恢复启用" : disabledStatus === "paused" ? "暂停" : "停用"}</button>}{deletable && !protectedRow && <button type="button" className="menu-action--danger" onClick={() => setDeleteTarget(item)}><Trash2 size={15} />删除</button>}</RowMenu>
          </td></tr>;
        })}</tbody></table></div>
      )}
    </div>
    <Drawer wide={fields.some((field) => field.type === "route-rules" || field.type === "outbound-config")} open={open} title={editing ? `编辑${title}` : createLabel} description={editing ? `修改 ${editing.id} 的配置` : `创建新的${title}记录`} onClose={() => { if (!saving) setOpen(false); }}><form className="drawer-form" onSubmit={submit} aria-busy={saving} noValidate>{formError && <div className="form-error" role="alert">{formError}</div>}{fields.filter((field) => !field.editOnly || editing).map((field) => <Field group={field.type === "node-picker" || field.type === "route-rules" || field.type === "outbound-config"} key={field.key} label={field.label} required={field.required} helper={field.helper} error={errors[field.key]}>{field.type === "textarea" || field.type === "json" ? <textarea className={field.type === "json" ? "code-editor code-editor--small" : ""} rows={field.type === "json" ? 9 : 4} value={values[field.key] ?? ""} placeholder={field.placeholder} onChange={(event) => updateValue(field.key, event.target.value)} /> : field.type === "select" ? <select disabled={Boolean(editing && field.lockedValue && values[field.key] === field.lockedValue)} value={values[field.key] ?? ""} onChange={(event) => updateValue(field.key, event.target.value)}>{field.options?.filter((option) => !option.exclusive || values[field.key] === option.value).map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select> : field.type === "node-picker" ? <NodeMultiSelect value={values[field.key] ?? ""} options={field.options ?? []} onChange={(value) => updateValue(field.key, value)} /> : field.type === "route-rules" ? <RouteRulesEditor value={values[field.key] ?? "[]"} outbounds={field.routeOutbounds ?? []} onChange={(value) => updateValue(field.key, value)} /> : field.type === "outbound-config" ? <OutboundConfigForm protocol={values.protocol ?? "socks"} settings={parseOutboundSettings(values.settings ?? "{}")} proxyTag={values.proxy_tag ?? ""} outbounds={field.routeOutbounds ?? []} currentID={editing?.id} onProtocolChange={(protocol, settings) => { setValues((current) => ({ ...current, protocol, settings: JSON.stringify(settings) })); setErrors((current) => ({ ...current, [field.key]: "" })); }} onSettingsChange={(settings) => updateValue("settings", JSON.stringify(settings))} onProxyTagChange={(value) => updateValue("proxy_tag", value)} /> : <input type={field.type ?? "text"} value={values[field.key] ?? ""} placeholder={field.placeholder} onChange={(event) => updateValue(field.key, event.target.value)} />}</Field>)}<footer className="drawer__actions"><Button type="button" variant="ghost" disabled={saving} onClick={() => setOpen(false)}>取消</Button><Button type="button" variant="primary" loading={saving} onClick={() => void save()}>{saving ? editing ? "保存中…" : "创建中…" : editing ? "保存修改" : "创建"}</Button></footer></form></Drawer>
    <Drawer open={Boolean(install)} title="一键安装 Corade Agent" description={install ? `目标服务器：${install.name}` : undefined} onClose={() => setInstall(null)}>{install && <div className="install-command"><p>在目标服务器的 root shell 中执行以下命令。</p><textarea className="code-editor" readOnly rows={7} value={install.command} /><footer><Button variant="primary" onClick={() => void writeClipboard(install.command).then(() => toast("安装命令已复制")).catch(() => toast("复制失败，请手动选择命令", "error"))}><Copy size={16} />复制命令</Button></footer></div>}</Drawer>
    <ConfirmDialog open={Boolean(deleteTarget)} title={`删除${subscriptionActions ? "这个账号" : `这条${title}记录`}？`} description={deleteTarget ? subscriptionActions ? `“${resourceName(deleteTarget)}”将从订阅账号列表中移除，现有订阅链接会立即失效。流量与审计历史仍会保留。` : `“${resourceName(deleteTarget)}”将从${title}列表中移除。存在关联资源时系统会阻止删除，审计历史仍会保留。` : ""} confirmLabel="确认删除" loading={deleting} onClose={() => setDeleteTarget(null)} onConfirm={() => void deleteResource()} />
    <ConfirmDialog open={Boolean(upgradeTarget)} title="升级 Agent" description={upgradeTarget ? `将在 ${resourceName(upgradeTarget)} 的下一次心跳中领取升级任务，并从 GitHub 更新至 ${latestAgentVersion(upgradeTarget)}。当前 Agent 会在独立任务中重启。` : ""} confirmLabel="下发升级任务" confirmVariant="primary" loading={upgrading} onClose={() => { if (!upgrading) setUpgradeTarget(null); }} onConfirm={() => void requestUpgrade()} />
  </div>;
}

const primary = (name: string, id: string, detail?: string) => <><strong className="cell-primary">{name}</strong><span className="resource-id">{detail || id}</span></>;
const subscriptionAvailable = (item: Resource) => "subscription_available" in item && item.subscription_available === true;
const resourceName = (item: Resource) => "name" in item && typeof item.name === "string" ? item.name : item.id;
type AgentUpgradeState = "disconnected" | "pending" | "running" | "latest" | "available" | "unknown";
const latestAgentVersion = (item: Resource) => "latest_agent_version" in item && typeof item.latest_agent_version === "string" ? item.latest_agent_version : "";
const agentUpgradeState = (item: Resource): AgentUpgradeState => {
  if (!("last_heartbeat_at" in item) || !item.last_heartbeat_at) return "disconnected";
  if ("agent_upgrade_task_id" in item && item.agent_upgrade_task_id) return "agent_upgrade_dispatched_at" in item && item.agent_upgrade_dispatched_at ? "running" : "pending";
  const latest = latestAgentVersion(item);
  if (!latest || !("agent_version" in item) || typeof item.agent_version !== "string") return "unknown";
  return item.agent_version.toLowerCase() === latest.toLowerCase() ? "latest" : "available";
};
const canRequestUpgrade = (item: Resource) => agentUpgradeState(item) === "available";
const upgradeStateLabel = (item: Resource) => ({ disconnected: "Agent 尚未连接", pending: "等待下一次心跳", running: "升级执行中，等待心跳确认", latest: "Agent 已是最新版本", available: `更新至 ${latestAgentVersion(item)}`, unknown: "暂时无法检查最新版本" })[agentUpgradeState(item)];
async function writeClipboard(value: string) {
  if (navigator.clipboard?.writeText) {
    try { await navigator.clipboard.writeText(value); return; } catch { /* HTTP deployments use the fallback below. */ }
  }
  const input = document.createElement("textarea");
  input.value = value;
  input.style.position = "fixed";
  input.style.opacity = "0";
  document.body.appendChild(input);
  input.select();
  const copied = document.execCommand("copy");
  input.remove();
  if (!copied) throw new Error("clipboard unavailable");
}
const localDateTime = (value: string | null) => value ? new Date(new Date(value).getTime() - new Date(value).getTimezoneOffset() * 60000).toISOString().slice(0, 16) : "";

export function MachinesPage() {
  return <ResourcePage<Machine>
    title="服务器"
    description="管理安装 Corade Agent 的物理机或虚拟机"
    endpoint="/machines"
    createLabel="添加服务器"
    installable
    upgradeable
    refreshIntervalMs={15_000}
    enabledStatus="pending"
    defaults={{ name: "", region: "", host: "", labels: "{}", notes: "" }}
    fields={[{ key: "name", label: "服务器名称", required: true, placeholder: "例如：香港边缘 01" }, { key: "region", label: "区域", placeholder: "例如：香港" }, { key: "host", label: "IP 或域名", placeholder: "hk01.example.net" }, { key: "labels", label: "标签 JSON", type: "json", helper: "用于筛选和自动化识别", placeholder: "{}" }, { key: "notes", label: "备注", type: "textarea" }]}
    transform={(v) => ({ name: v.name, region: v.region, host: v.host, labels: JSON.parse(v.labels || "{}"), notes: v.notes })}
    toValues={(item) => ({ name: item.name, region: item.region, host: item.host, labels: JSON.stringify(item.labels ?? {}, null, 2), notes: item.notes ?? "" })}
    columns={[
      { label: "服务器", render: (item) => primary(item.name, item.id, item.host || item.id) },
      { label: "区域", render: (item) => item.region || "未设置" },
      { label: "节点", render: (item) => <span className="mono">{item.node_count}</span> },
      { label: "资源占用", render: (item) => <MachineMetrics machine={item} /> },
      { label: "Agent", render: (item) => <><span className="cell-primary">{item.agent_version || "未接入"}</span><span className={`resource-id ${item.agent_protocol === "v2" ? "text-success" : ""}`}>{item.agent_protocol === "v2" ? "V2 加密" : "旧通讯"} · {agentUpgradeLabel(item)}</span></> },
      { label: "最后心跳", render: (item) => formatPreciseDate(item.last_heartbeat_at) },
    ]}
  />;
}

function MachineMetrics({ machine }: { machine: Machine }) {
  const memory = usagePercent(machine.metrics?.mem?.used, machine.metrics?.mem?.total);
  const disk = usagePercent(machine.metrics?.disk?.used, machine.metrics?.disk?.total);
  if (!machine.metrics_sampled_at) return <span className="history-muted">等待心跳</span>;
  return <div className="machine-metrics" title={`采样时间 ${formatPreciseDate(machine.metrics_sampled_at)}`}>
    <span><b>CPU</b>{formatPercent(machine.metrics?.cpu)}</span>
    <span><b>内存</b>{formatPercent(memory)}</span>
    <span><b>磁盘</b>{formatPercent(disk)}</span>
  </div>;
}

function usagePercent(used?: number, total?: number) {
  return typeof used === "number" && typeof total === "number" && total > 0 ? used / total * 100 : undefined;
}

function formatPercent(value?: number) {
  return typeof value === "number" && Number.isFinite(value) ? `${value.toLocaleString("zh-CN", { maximumFractionDigits: 1 })}%` : "--";
}

function TrafficUsage({ used, limit }: { used: number; limit: number }) {
  const percent = limit > 0 ? Math.min(100, Math.max(0, used / limit * 100)) : 0;
  return <div className="traffic-usage">
    <span><strong>{formatBytes(used)}</strong><small>/ {limit > 0 ? formatBytes(limit) : "不限"}</small></span>
    {limit > 0 && <span className="traffic-usage__track" role="progressbar" aria-label="流量使用比例" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(percent)}><i style={{ width: `${percent}%` }} /></span>}
  </div>;
}

function agentUpgradeLabel(machine: Machine) {
  const state = agentUpgradeState(machine);
  if (state === "available") return `可更新至 ${machine.latest_agent_version}`;
  return ({ disconnected: "尚未连接", pending: "升级任务等待心跳", running: "升级执行中", latest: "已是最新版本", unknown: "版本检查暂不可用" } as const)[state];
}

export function AccessGroupsPage() {
  const { data: nodes } = useResource<Node[]>("/nodes", []);
  return <ResourcePage<AccessGroup> title="权限组" description="控制用户和朋友可以访问的节点范围" endpoint="/access-groups" createLabel="添加权限组" defaults={{ name: "", notes: "", node_ids: "" }} fields={[{ key: "name", label: "权限组名称", required: true, placeholder: "例如：标准线路" }, { key: "node_ids", label: "可访问节点", type: "node-picker", options: nodes.map((item) => ({ value: item.id, label: item.name, detail: `${item.protocol.toUpperCase()} · ${item.machine_name} · 端口 ${item.server_port}`, keywords: `${item.id} ${item.machine_name} ${item.protocol} ${item.server_port}` })) }, { key: "notes", label: "备注", type: "textarea", placeholder: "选填，仅管理员可见" }]} transform={(v) => ({ name: v.name, notes: v.notes, node_ids: v.node_ids.split(",").filter(Boolean) })} toValues={(item) => ({ name: item.name, notes: item.notes ?? "", node_ids: (item.node_ids ?? []).join(",") })} columns={[{ label: "权限组", render: (item) => primary(item.name, item.id, item.notes) }, { label: "节点数", render: (item) => <span className="mono">{item.node_count}</span> }, { label: "账号数", render: (item) => <span className="mono">{item.user_count}</span> }, { label: "更新时间", render: (item) => formatDate(item.updated_at) }]} />;
}

export function PlansPage() {
  const { data: groups } = useResource<AccessGroup[]>("/access-groups", []);
  return <ResourcePage<Plan> title="套餐" description="定义流量、速率、设备数和默认访问范围" endpoint="/plans" createLabel="添加套餐" defaults={{ name: "", access_group_id: groups[0]?.id ?? "", traffic_gb: "500", speed_limit_mbps: "100", device_limit: "3", default_valid_days: "30", notes: "" }} fields={[{ key: "name", label: "套餐名称", required: true }, { key: "access_group_id", label: "默认权限组", type: "select", required: true, options: [{ value: "", label: "请选择权限组" }, ...groups.map((item) => ({ value: item.id, label: item.name }))] }, { key: "traffic_gb", label: "每月流量（GB）", type: "number", required: true }, { key: "speed_limit_mbps", label: "速率限制（Mbps）", type: "number" }, { key: "device_limit", label: "同时在线设备数", type: "number" }, { key: "default_valid_days", label: "默认有效天数", type: "number" }, { key: "notes", label: "备注", type: "textarea" }]} transform={(v) => ({ name: v.name, access_group_id: v.access_group_id, traffic_limit_bytes: Number(v.traffic_gb) * 1024 ** 3, speed_limit_mbps: Number(v.speed_limit_mbps || 0), device_limit: Number(v.device_limit || 0), default_valid_days: Number(v.default_valid_days || 0), notes: v.notes })} toValues={(item) => ({ name: item.name, access_group_id: item.access_group_id, traffic_gb: String(item.traffic_limit_bytes / 1024 ** 3), speed_limit_mbps: String(item.speed_limit_mbps), device_limit: String(item.device_limit), default_valid_days: String(item.default_valid_days), notes: item.notes ?? "" })} columns={[{ label: "套餐", render: (item) => primary(item.name, item.id, item.access_group_name) }, { label: "流量", render: (item) => formatBytes(item.traffic_limit_bytes) }, { label: "速率", render: (item) => `${item.speed_limit_mbps || "不限"}${item.speed_limit_mbps ? " Mbps" : ""}` }, { label: "设备", render: (item) => item.device_limit || "不限" }, { label: "账号数", render: (item) => <span className="mono">{item.user_count}</span> }, { label: "重置", render: () => "自然月" }]} />;
}

export function UsersPage() {
  const { data: plans } = useResource<Plan[]>("/plans", []);
  return <ResourcePage<User>
    title="用户、朋友与管理员"
    description="管理员可登录后台并使用订阅；用户与朋友仅能使用订阅"
    endpoint="/users"
    createLabel="添加用户或朋友"
    subscriptionActions
    quickAccountCreation
    deletable
    disabledStatus="paused"
    protectedItem={(item) => item.role === "admin"}
    defaults={{ role: "user", name: "", email: "", plan_id: plans[0]?.id ?? "", expires_at: "", notes: "" }}
    fields={[
      { key: "role", label: "账号分类", type: "select", lockedValue: "admin", options: [{ value: "admin", label: "管理员", exclusive: true }, { value: "user", label: "用户" }, { value: "friend", label: "朋友" }] },
      { key: "uuid", label: "UDID", editOnly: true, required: true, helper: "修改后会随节点成员配置同步到 Corade" },
      { key: "name", label: "账号名称", required: true },
      { key: "email", label: "邮箱", type: "email" },
      { key: "plan_id", label: "套餐", type: "select", options: [{ value: "", label: "暂不分配" }, ...plans.map((item) => ({ value: item.id, label: item.name }))] },
      { key: "expires_at", label: "到期时间", type: "datetime-local" },
      { key: "notes", label: "内部备注", type: "textarea" },
    ]}
    transform={(v, editing) => {
      const email = v.email.trim();
      const values = { role: v.role, name: v.name.trim(), email: editing ? email : email || null, plan_id: editing ? v.plan_id : v.plan_id || null, expires_at: v.expires_at ? new Date(v.expires_at).toISOString() : editing ? "" : null, notes: v.notes };
      return editing ? { ...values, uuid: v.uuid } : values;
    }}
    toValues={(item) => ({ role: item.role, uuid: item.uuid, name: item.name, email: item.email ?? "", plan_id: item.plan_id ?? "", expires_at: localDateTime(item.expires_at), notes: item.notes ?? "" })}
    columns={[
      { label: "账号", render: (item) => primary(item.name, item.id, item.email || item.id) },
      { label: "分类", render: (item) => item.role === "admin" ? "管理员" : item.role === "friend" ? "朋友" : "用户" },
      { label: "套餐", render: (item) => item.plan_name || "未分配" },
      { label: "流量用量", render: (item) => <TrafficUsage used={item.traffic_used_bytes} limit={item.traffic_limit_bytes} /> },
      { label: "到期", render: (item) => item.expires_at ? formatDateWithYear(item.expires_at) : "长期" },
    ]}
  />;
}

export function RoutesPage() {
  const { data: outbounds } = useResource<Outbound[]>("/outbounds", []);
  const initialRules = JSON.stringify([{ name: "", match: { domain_suffixes: [] }, action: { type: "direct" } }]);
  return <ResourcePage<RoutePolicy> title="路由策略" description="按顺序匹配域名、IP、端口与网络协议，并执行直连、阻断或指定出站" endpoint="/route-policies" createLabel="添加路由策略" defaults={{ name: "", notes: "", rules: initialRules }} fields={[{ key: "name", label: "策略名称", required: true, placeholder: "例如：流媒体直连" }, { key: "notes", label: "策略备注", type: "textarea", placeholder: "选填，例如适用区域或变更原因" }, { key: "rules", label: "规则编排", type: "route-rules", routeOutbounds: outbounds }]} transform={(v) => ({ name: v.name.trim(), notes: v.notes.trim(), rules: JSON.parse(v.rules || "[]") })} toValues={(item) => ({ name: item.name, notes: item.notes ?? "", rules: JSON.stringify(item.rules ?? []) })} columns={[{ label: "策略", render: (item) => primary(item.name, item.id, item.notes) }, { label: "规则", render: (item) => <span className="mono">{item.rules?.length ?? 0}</span> }, { label: "当前版本", render: (item) => <span className="mono">rev {item.current_revision}</span> }, { label: "绑定节点", render: (item) => <span className="mono">{item.node_count}</span> }, { label: "更新时间", render: (item) => formatDate(item.updated_at) }]} />;
}

export function OutboundsPage() {
  const { data: outbounds } = useResource<Outbound[]>("/outbounds", []);
  return <ResourcePage<Outbound> title="出站" description="维护路由策略可选择的 Xray 代理出站" endpoint="/outbounds" createLabel="添加出站" defaults={{ name: "", tag: "", protocol: "socks", proxy_tag: "", settings: JSON.stringify(defaultOutboundSettings("socks")), kernel_support: "xray" }} fields={[{ key: "name", label: "出站名称", required: true, placeholder: "例如：新加坡 SOCKS 出站" }, { key: "tag", label: "唯一标记", required: true, placeholder: "sg-socks" }, { key: "settings", label: "协议与连接参数", type: "outbound-config", required: true, routeOutbounds: outbounds }]} transform={(v) => ({ name: v.name.trim(), tag: v.tag.trim(), protocol: v.protocol, proxy_tag: v.proxy_tag, settings: JSON.parse(v.settings || "{}"), kernel_support: ["xray"] })} toValues={(item) => ({ name: item.name, tag: item.tag, protocol: item.protocol, proxy_tag: item.proxy_tag, settings: JSON.stringify(item.settings ?? {}), kernel_support: (item.kernel_support ?? ["xray"]).join(",") })} columns={[{ label: "出站", render: (item) => primary(item.name, item.id, item.tag) }, { label: "协议", render: (item) => <span className="protocol-label">{item.protocol}</span> }, { label: "上游", render: (item) => item.proxy_tag || "无" }, { label: "内核", render: (item) => item.kernel_support.map((kernel) => kernel === "xray" ? "Xray" : kernel).join(" / ") }, { label: "更新时间", render: (item) => formatDate(item.updated_at) }]} />;
}
