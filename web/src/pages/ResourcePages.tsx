import { useMemo, useState, type FormEvent, type ReactNode } from "react";
import { Copy, Edit3, Link2, Plus, Power, RefreshCw, Search, Terminal, Trash2, UserPlus, UsersRound } from "lucide-react";
import { ApiError, api } from "../api";
import { Button, ConfirmDialog, Drawer, EmptyState, Field, PageHeader, RowMenu, StatusBadge, TableSkeleton, formatBytes, formatDate, useToast } from "../components/ui";
import { useResource } from "../hooks";
import type { AccessGroup, Machine, Node, Outbound, Plan, RoutePolicy, User } from "../types";

type Resource = { id: string; status: string; created_at: string; updated_at: string };
type Column<T> = { label: string; render: (item: T) => ReactNode; className?: string };
type FieldSpec = {
  key: string;
  label: string;
  type?: "text" | "number" | "textarea" | "select" | "multiselect" | "json" | "email" | "datetime-local";
  required?: boolean;
  helper?: string;
  options?: { value: string; label: string; exclusive?: boolean }[];
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
};

function ResourcePage<T extends Resource>({ title, description, endpoint, createLabel, columns, fields, defaults, transform, toValues, enabledStatus = "active", disabledStatus = "disabled", installable = false, upgradeable = false, subscriptionActions = false, quickAccountCreation = false, deletable = false, identifierAction, protectedItem }: ResourcePageProps<T>) {
	deletable = deletable || ["/machines", "/access-groups", "/plans", "/route-policies"].includes(endpoint);
	if (endpoint === "/route-policies") enabledStatus = "published";
  const { data, loading, error, reload } = useResource<T[]>(endpoint, []);
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
            {upgradeable && <button type="button" className="icon-button" aria-label={`升级 ${resourceName(item)} 的 Agent`} title={upgradeStateLabel(item)} disabled={!canRequestUpgrade(item) || upgrading} onClick={() => setUpgradeTarget(item)}><RefreshCw size={16} /></button>}
            <button type="button" className="icon-button" aria-label={`编辑 ${item.id}`} title="编辑" onClick={() => showEdit(item)}><Edit3 size={16} /></button>
            {deletable && !protectedRow && <button type="button" className="icon-button icon-button--danger" aria-label={`删除 ${resourceName(item)}`} title="删除" onClick={() => setDeleteTarget(item)}><Trash2 size={16} /></button>}
            <RowMenu label={`${item.id} 更多操作`}><button type="button" onClick={() => showEdit(item)}><Edit3 size={15} />编辑</button>{identifierAction && <button type="button" disabled={copyingIdentifier === item.id} onClick={() => void copyIdentifier(item)}><Copy size={15} />复制 {identifierAction.label}</button>}{subscriptionActions && <button type="button" disabled={!canCopy || copying === item.id} onClick={() => void copySubscription(item)}><Link2 size={15} />{canCopy ? "复制订阅链接" : "完整令牌未保存"}</button>}{installable && <button type="button" onClick={() => void loadInstallation(item)}><Terminal size={15} />一键安装 Agent</button>}{!protectedRow && <button type="button" disabled={changingStatus === item.id} onClick={() => void toggleStatus(item)}><Power size={15} />{item.status === disabledStatus ? "恢复启用" : disabledStatus === "paused" ? "暂停" : "停用"}</button>}{deletable && !protectedRow && <button type="button" className="menu-action--danger" onClick={() => setDeleteTarget(item)}><Trash2 size={15} />删除</button>}</RowMenu>
          </td></tr>;
        })}</tbody></table></div>
      )}
    </div>
    <Drawer open={open} title={editing ? `编辑${title}` : createLabel} description={editing ? `修改 ${editing.id} 的配置` : `创建新的${title}记录`} onClose={() => { if (!saving) setOpen(false); }}><form className="drawer-form" onSubmit={submit} aria-busy={saving} noValidate>{formError && <div className="form-error" role="alert">{formError}</div>}{fields.filter((field) => !field.editOnly || editing).map((field) => <Field key={field.key} label={field.label} required={field.required} helper={field.helper} error={errors[field.key]}>{field.type === "textarea" || field.type === "json" ? <textarea className={field.type === "json" ? "code-editor code-editor--small" : ""} rows={field.type === "json" ? 9 : 4} value={values[field.key] ?? ""} placeholder={field.placeholder} onChange={(event) => updateValue(field.key, event.target.value)} /> : field.type === "select" ? <select disabled={Boolean(editing && field.lockedValue && values[field.key] === field.lockedValue)} value={values[field.key] ?? ""} onChange={(event) => updateValue(field.key, event.target.value)}>{field.options?.filter((option) => !option.exclusive || values[field.key] === option.value).map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select> : field.type === "multiselect" ? <select multiple size={Math.min(8, Math.max(3, field.options?.length ?? 3))} value={(values[field.key] ?? "").split(",").filter(Boolean)} onChange={(event) => updateValue(field.key, Array.from(event.target.selectedOptions).map((option) => option.value).join(","))}>{field.options?.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select> : <input type={field.type ?? "text"} value={values[field.key] ?? ""} placeholder={field.placeholder} onChange={(event) => updateValue(field.key, event.target.value)} />}</Field>)}<footer className="drawer__actions"><Button type="button" variant="ghost" disabled={saving} onClick={() => setOpen(false)}>取消</Button><Button type="button" variant="primary" loading={saving} onClick={() => void save()}>{saving ? editing ? "保存中…" : "创建中…" : editing ? "保存修改" : "创建"}</Button></footer></form></Drawer>
    <Drawer open={Boolean(install)} title="一键安装 Corade Agent" description={install ? `目标服务器：${install.name}` : undefined} onClose={() => setInstall(null)}>{install && <div className="install-command"><p>在目标服务器的 root shell 中执行以下命令。</p><textarea className="code-editor" readOnly rows={7} value={install.command} /><footer><Button variant="primary" onClick={() => void writeClipboard(install.command).then(() => toast("安装命令已复制")).catch(() => toast("复制失败，请手动选择命令", "error"))}><Copy size={16} />复制命令</Button></footer></div>}</Drawer>
    <ConfirmDialog open={Boolean(deleteTarget)} title={`删除${subscriptionActions ? "这个账号" : `这条${title}记录`}？`} description={deleteTarget ? subscriptionActions ? `“${resourceName(deleteTarget)}”将从订阅账号列表中移除，现有订阅链接会立即失效。流量与审计历史仍会保留。` : `“${resourceName(deleteTarget)}”将从${title}列表中移除。存在关联资源时系统会阻止删除，审计历史仍会保留。` : ""} confirmLabel="确认删除" loading={deleting} onClose={() => setDeleteTarget(null)} onConfirm={() => void deleteResource()} />
    <ConfirmDialog open={Boolean(upgradeTarget)} title="升级 Agent" description={upgradeTarget ? `将在 ${resourceName(upgradeTarget)} 的下一次心跳中领取升级任务，并从 GitHub 安装最新版本。当前 Agent 会在独立任务中重启。` : ""} confirmLabel="下发升级任务" confirmVariant="primary" loading={upgrading} onClose={() => { if (!upgrading) setUpgradeTarget(null); }} onConfirm={() => void requestUpgrade()} />
  </div>;
}

const primary = (name: string, id: string, detail?: string) => <><strong className="cell-primary">{name}</strong><span className="resource-id">{detail || id}</span></>;
const subscriptionAvailable = (item: Resource) => "subscription_available" in item && item.subscription_available === true;
const resourceName = (item: Resource) => "name" in item && typeof item.name === "string" ? item.name : item.id;
const canRequestUpgrade = (item: Resource) => "last_heartbeat_at" in item && Boolean(item.last_heartbeat_at) && !("agent_upgrade_task_id" in item && item.agent_upgrade_task_id && !("agent_upgrade_dispatched_at" in item && item.agent_upgrade_dispatched_at));
const upgradeStateLabel = (item: Resource) => !("last_heartbeat_at" in item) || !item.last_heartbeat_at ? "Agent 尚未连接" : "agent_upgrade_task_id" in item && item.agent_upgrade_task_id && !("agent_upgrade_dispatched_at" in item && item.agent_upgrade_dispatched_at) ? "等待下一次心跳" : "升级 Agent";
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
      { label: "Agent", render: (item) => <><span className="cell-primary">{item.agent_version || "未接入"}</span><span className="resource-id">{agentUpgradeLabel(item)}</span></> },
      { label: "最后心跳", render: (item) => formatDate(item.last_heartbeat_at) },
    ]}
  />;
}

function MachineMetrics({ machine }: { machine: Machine }) {
  const memory = usagePercent(machine.metrics?.mem?.used, machine.metrics?.mem?.total);
  const disk = usagePercent(machine.metrics?.disk?.used, machine.metrics?.disk?.total);
  if (!machine.metrics_sampled_at) return <span className="history-muted">等待心跳</span>;
  return <div className="machine-metrics" title={`采样时间 ${formatDate(machine.metrics_sampled_at)}`}>
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

function agentUpgradeLabel(machine: Machine) {
  if (!machine.last_heartbeat_at) return "尚未连接";
  if (machine.agent_upgrade_task_id && !machine.agent_upgrade_dispatched_at) return "升级任务等待心跳";
  if (machine.agent_upgrade_dispatched_at) return "升级任务已下发";
  return "可一键升级";
}

export function AccessGroupsPage() {
  const { data: nodes } = useResource<Node[]>("/nodes", []);
  return <ResourcePage<AccessGroup> title="权限组" description="控制用户和朋友可以访问的节点范围" endpoint="/access-groups" createLabel="添加权限组" defaults={{ name: "", notes: "", node_ids: "" }} fields={[{ key: "name", label: "权限组名称", required: true, placeholder: "例如：标准线路" }, { key: "node_ids", label: "可访问节点", type: "multiselect", helper: "可按住 Ctrl 或 Command 选择多个节点", options: nodes.map((item) => ({ value: item.id, label: `${item.name} · ${item.protocol}:${item.server_port}` })) }, { key: "notes", label: "备注", type: "textarea" }]} transform={(v) => ({ name: v.name, notes: v.notes, node_ids: v.node_ids.split(",").filter(Boolean) })} toValues={(item) => ({ name: item.name, notes: item.notes ?? "", node_ids: (item.node_ids ?? []).join(",") })} columns={[{ label: "权限组", render: (item) => primary(item.name, item.id, item.notes) }, { label: "节点数", render: (item) => <span className="mono">{item.node_count}</span> }, { label: "账号数", render: (item) => <span className="mono">{item.user_count}</span> }, { label: "更新时间", render: (item) => formatDate(item.updated_at) }]} />;
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
    identifierAction={{ label: "UDID", value: (item) => item.uuid }}
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
      { label: "UDID", className: "col-udid", render: (item) => <span className="mono data-ellipsis" title={item.uuid}>{item.uuid}</span> },
      { label: "套餐", render: (item) => item.plan_name || "未分配" },
      { label: "已用流量", render: (item) => formatBytes(item.traffic_used_bytes) },
      { label: "订阅令牌", render: (item) => item.subscription_available ? <span className="mono">{item.subscription_token_prefix}…</span> : <span className="token-unavailable">未保存完整令牌</span> },
      { label: "到期", render: (item) => item.expires_at ? formatDate(item.expires_at) : "长期" },
    ]}
  />;
}

export function RoutesPage() {
  return <ResourcePage<RoutePolicy> title="路由策略" description="维护可复用的匹配规则与出站动作" endpoint="/route-policies" createLabel="添加路由策略" enabledStatus="draft" defaults={{ name: "", notes: "" }} fields={[{ key: "name", label: "策略名称", required: true }, { key: "notes", label: "备注", type: "textarea" }]} transform={(v) => ({ name: v.name, notes: v.notes })} toValues={(item) => ({ name: item.name, notes: item.notes ?? "" })} columns={[{ label: "策略", render: (item) => primary(item.name, item.id, item.notes) }, { label: "当前版本", render: (item) => <span className="mono">rev {item.current_revision}</span> }, { label: "绑定节点", render: (item) => <span className="mono">{item.node_count}</span> }, { label: "更新时间", render: (item) => formatDate(item.updated_at) }]} />;
}

export function OutboundsPage() {
  return <ResourcePage<Outbound> title="出站" description="定义直连、阻断或代理转发目标" endpoint="/outbounds" createLabel="添加出站" defaults={{ name: "", tag: "", protocol: "direct", proxy_tag: "", settings: "{}" }} fields={[{ key: "name", label: "出站名称", required: true }, { key: "tag", label: "唯一标记", required: true, placeholder: "sg-proxy" }, { key: "protocol", label: "协议", type: "select", options: ["direct", "block", "vless", "vmess", "trojan", "shadowsocks", "socks", "http", "wireguard"].map((value) => ({ value, label: value })) }, { key: "proxy_tag", label: "上游出站标记", helper: "用于链式出站，可留空" }, { key: "settings", label: "连接设置 JSON", type: "json", required: true }]} transform={(v) => ({ name: v.name, tag: v.tag, protocol: v.protocol, proxy_tag: v.proxy_tag, settings: JSON.parse(v.settings || "{}"), kernel_support: ["singbox", "xray"] })} toValues={(item) => ({ name: item.name, tag: item.tag, protocol: item.protocol, proxy_tag: item.proxy_tag, settings: JSON.stringify(item.settings ?? {}, null, 2) })} columns={[{ label: "出站", render: (item) => primary(item.name, item.id, item.tag) }, { label: "协议", render: (item) => <span className="protocol-label">{item.protocol}</span> }, { label: "上游", render: (item) => item.proxy_tag || "无" }, { label: "内核", render: (item) => item.kernel_support.join(" / ") }, { label: "更新时间", render: (item) => formatDate(item.updated_at) }]} />;
}
