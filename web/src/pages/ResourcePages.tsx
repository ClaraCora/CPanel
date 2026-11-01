import { useEffect, useMemo, useState, type FormEvent, type ReactNode } from "react";
import { Copy, Edit3, Filter, KeyRound, Link2, Plus, Power, RefreshCw, Search, SquareArrowOutUpRight, Terminal, Trash2, UserPlus, UsersRound } from "lucide-react";
import { ApiError, api } from "../api";
import { Button, ConfirmDialog, DataFreshness, Drawer, EmptyState, Field, PageHeader, RowMenu, StatusBadge, TableSkeleton, formatBytes, formatDate, formatDateWithYear, formatPreciseDate, useToast } from "../components/ui";
import { NodeMultiSelect, type MultiSelectOption } from "../components/NodeMultiSelect";
import { OutboundConfigForm, defaultOutboundSettings, parseOutboundSettings, validateOutboundSettingsValue } from "../components/OutboundConfigForm";
import { RouteRulesEditor, validateRouteRulesValue } from "../components/RouteRulesEditor";
import { ResourceCategoryFilters, ResourceDate, ResourceListFooter, ResourceListNote, ResourceListSurface, ResourceRowActions } from "../components/ResourceList";
import { useResource } from "../hooks";
import type { AccessGroup, Machine, Node, Outbound, Plan, RoutePolicy, User } from "../types";

type Resource = { id: string; status: string; created_at: string; updated_at: string };
type Column<T> = { label: string; render: (item: T) => ReactNode; className?: string; width?: string };

type ClearListOptions<T> = {
  kind: "machines" | "users" | "plans" | "groups" | "routes" | "outbounds";
  searchPlaceholder: string;
  note: string;
  categories?: { value: string; label: string; matches: (item: T) => boolean }[];
  filter?: { label: string; allLabel: string; value: (item: T) => string };
};
type FieldSpec = {
  key: string;
  label: string;
  type?: "text" | "number" | "textarea" | "select" | "node-picker" | "route-rules" | "outbound-config" | "json" | "email" | "datetime-local" | "password";
  required?: boolean;
  helper?: string;
  options?: (MultiSelectOption & { exclusive?: boolean })[];
	routeOutbounds?: Outbound[];
  placeholder?: string;
  editOnly?: boolean;
  lockedValue?: string;
	disabled?: (item: any | null) => boolean;
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
  identityResettable?: boolean;
  subscriptionActions?: boolean;
	portalActions?: boolean;
  quickAccountCreation?: boolean;
  deletable?: boolean;
  identifierAction?: { label: string; value: (item: T) => string };
  protectedItem?: (item: T) => boolean;
  refreshIntervalMs?: number;
  list?: ClearListOptions<T>;
};

function ResourcePage<T extends Resource>({ title, description, endpoint, createLabel, columns, fields, defaults, transform, toValues, enabledStatus = "active", disabledStatus = "disabled", installable = false, upgradeable = false, identityResettable = false, subscriptionActions = false, portalActions = false, quickAccountCreation = false, deletable = false, identifierAction, protectedItem, refreshIntervalMs = 0, list }: ResourcePageProps<T>) {
	deletable = deletable || ["/machines", "/access-groups", "/plans", "/route-policies", "/outbounds"].includes(endpoint);
	if (endpoint === "/route-policies") enabledStatus = "published";
  const { data, loading, refreshing, error, refreshError, lastSuccessAt, stale, reload } = useResource<T[]>(endpoint, [], refreshIntervalMs);
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [category, setCategory] = useState("all");
  const [detailFilter, setDetailFilter] = useState("");
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<T | null>(null);
  const [values, setValues] = useState(defaults);
  const [initialValues, setInitialValues] = useState(defaults);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [changingStatus, setChangingStatus] = useState("");
  const [copying, setCopying] = useState("");
	const [openingPortal, setOpeningPortal] = useState("");
  const [copyingIdentifier, setCopyingIdentifier] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<T | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [install, setInstall] = useState<{ name: string; command: string } | null>(null);
  const [upgradeTarget, setUpgradeTarget] = useState<T | null>(null);
  const [upgrading, setUpgrading] = useState(false);
  const [identityResetTarget, setIdentityResetTarget] = useState<T | null>(null);
  const [resettingIdentity, setResettingIdentity] = useState(false);
  const [formError, setFormError] = useState("");
  const [discardEditor, setDiscardEditor] = useState(false);
  const [quickCreating, setQuickCreating] = useState<"user" | "friend" | "">("");
  const toast = useToast();
  const statusOptions = useMemo(() => Array.from(new Set(data.map((item) => item.status).filter(Boolean))), [data]);
  const categories = list?.categories ?? statusOptions.map((status) => ({ value: status, label: statusLabel(status), matches: (item: T) => item.status === status }));
  const detailOptions = list?.filter ? Array.from(new Set(data.map(list.filter.value).filter(Boolean))).sort((a, b) => a.localeCompare(b, "zh-CN")) : [];
  const filtered = useMemo(() => data.filter((item) => {
    const matchesQuery = !query.trim() || JSON.stringify(item).toLowerCase().includes(query.trim().toLowerCase());
    const matchesStatus = statusFilter === "all" || item.status === statusFilter;
    const matchesCategory = !list || category === "all" || (list.categories ? list.categories.find((option) => option.value === category)?.matches(item) : item.status === category);
    const matchesDetail = !list?.filter || !detailFilter || list.filter.value(item) === detailFilter;
    return matchesQuery && matchesStatus && matchesCategory && matchesDetail;
  }), [data, query, statusFilter, category, detailFilter, list]);
  const formDirty = open && !sameFormValues(values, initialValues);

  useEffect(() => {
    if (!formDirty) return;
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ""; };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [formDirty]);

  function requestCloseEditor() {
    if (saving) return;
    if (formDirty) {
      setDiscardEditor(true);
      return;
    }
    setOpen(false);
  }

  function showCreate() {
    setEditing(null);
    const nextValues = {
      ...defaults,
      ...(endpoint === "/plans" ? { speed_limit_mbps: "0" } : {}),
    };
    setValues(nextValues);
    setInitialValues(nextValues);
    setErrors({});
    setFormError("");
    setOpen(true);
  }

  function showEdit(item: T) {
    setEditing(item);
    const nextValues = toValues(item);
    setValues(nextValues);
    setInitialValues(nextValues);
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
        const message = requestErrorMessage(reason, "保存失败，请重试");
        setFormError(message);
        toast(message, "error");
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
      toast(requestErrorMessage(reason, "状态修改失败"), "error");
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
      toast(requestErrorMessage(reason, "订阅链接复制失败，请重试"), "error");
    } finally {
      setCopying("");
    }
  }

	async function openPortal(item: T) {
		setOpeningPortal(item.id);
		try {
			const result = await api.post<{ grant: string }>(`${endpoint}/${item.id}/portal`);
			window.open(`/edu?grant=${encodeURIComponent(result.grant)}`, "_blank", "noopener,noreferrer");
		} catch (reason) {
			toast(requestErrorMessage(reason, "订阅页面打开失败，请重试"), "error");
		} finally {
			setOpeningPortal("");
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
      toast(requestErrorMessage(reason, `${title}删除失败，请重试`), "error");
    } finally {
      setDeleting(false);
    }
  }

  async function loadInstallation(item: T) {
    try {
      const result = await api.post<{ command: string }>(`${endpoint}/${item.id}/installation`);
      setInstall({ name: "name" in item ? String(item.name) : item.id, command: result.command });
      toast("安装凭据仅限本机，30 分钟内有效且只能登记一次；重新生成会撤销旧命令");
    } catch (reason) {
      toast(requestErrorMessage(reason, "安装命令生成失败"), "error");
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

  async function resetAgentIdentity() {
    if (!identityResetTarget || resettingIdentity) return;
    setResettingIdentity(true);
    try {
      await api.delete(endpoint + "/" + identityResetTarget.id + "/agent-identity");
      toast("旧 Agent 身份已清除，请重新生成并执行一键安装命令");
      setIdentityResetTarget(null);
      await reload();
    } catch (reason) {
      toast(requestErrorMessage(reason, "Agent 身份重置失败，请重试"), "error");
    } finally {
      setResettingIdentity(false);
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
      toast(requestErrorMessage(reason, "一键添加失败，请重试"), "error");
    } finally {
      setQuickCreating("");
    }
  }

  function updateValue(key: string, value: string) {
    setValues((current) => ({ ...current, [key]: value }));
    setErrors((current) => ({ ...current, [key]: "" }));
  }

  return <div className={`page${list ? ` clear-resources clear-resources--${list.kind}` : ""}`}>
    <PageHeader title={title} description={description} actions={<>{quickAccountCreation && <><Button type="button" loading={quickCreating === "user"} disabled={Boolean(quickCreating)} onClick={() => void quickCreateAccount("user")}><UserPlus size={16} />{quickCreating === "user" ? "添加中…" : "一键添加用户"}</Button><Button type="button" loading={quickCreating === "friend"} disabled={Boolean(quickCreating)} onClick={() => void quickCreateAccount("friend")}><UsersRound size={16} />{quickCreating === "friend" ? "添加中…" : "一键添加朋友"}</Button></>}<Button type="button" variant="primary" disabled={Boolean(quickCreating)} onClick={showCreate}><Plus size={16} />{createLabel}</Button></>} />
    <ResourceListSurface enabled={Boolean(list)}>
    {list && <ResourceCategoryFilters label={`${title}分类筛选`} value={category} options={[{ value: "all", label: "全部", count: data.length }, ...categories.map((option) => ({ value: option.value, label: option.label, count: data.filter(option.matches).length }))]} onChange={setCategory} />}
    <div className="toolbar"><label className="search-box"><Search size={16} aria-hidden="true" /><span className="sr-only">搜索{title}</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={list?.searchPlaceholder ?? `搜索${title}`} /></label>{statusOptions.length > 1 && (!list || list.categories) && <div className="toolbar__filters"><Filter size={15} aria-hidden="true" /><label className="sr-only" htmlFor={`${endpoint.slice(1)}-status-filter`}>按状态筛选{title}</label><select id={`${endpoint.slice(1)}-status-filter`} value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}><option value="all">全部状态</option>{statusOptions.map((status) => <option value={status} key={status}>{statusLabel(status)}</option>)}</select></div>}{list?.filter && <label className="toolbar__filters"><span className="sr-only">{list.filter.label}</span><select value={detailFilter} onChange={(event) => setDetailFilter(event.target.value)}><option value="">{list.filter.allLabel}</option>{detailOptions.map((option) => <option value={option} key={option}>{option}</option>)}</select></label>}{refreshIntervalMs > 0 && <DataFreshness lastSuccessAt={lastSuccessAt} stale={stale} refreshing={refreshing} error={refreshError} />}<span className="toolbar__count">{filtered.length}{filtered.length !== data.length ? ` / ${data.length}` : ""} 条记录</span></div>
    <div className="table-surface">
      {loading ? <TableSkeleton columns={columns.length + 2} /> : error ? <EmptyState title={`${title}加载失败`} description={error} action={<Button onClick={() => void reload()}>重新加载</Button>} /> : filtered.length === 0 ? <EmptyState title={data.length ? "没有符合搜索条件的记录" : `还没有${title}`} description={data.length ? "更换搜索词或调整筛选条件后重试。" : `创建第一条${title}记录开始管理。`} action={data.length && list ? <Button onClick={() => { setQuery(""); setStatusFilter("all"); setCategory("all"); setDetailFilter(""); }}>清除筛选</Button> : !data.length && <Button variant="primary" onClick={showCreate}><Plus size={16} />{createLabel}</Button>} /> : (
        <div className="table-scroll resource-table-scroll"><table aria-label={`${title}列表`}>{list && <colgroup>{columns.map((column) => <col key={column.label} style={{ width: column.width }} />)}<col className="clear-status-col" /><col className="clear-actions-col" /></colgroup>}<thead><tr>{columns.map((column) => <th scope="col" className={column.className} key={column.label}>{column.label}</th>)}<th scope="col">状态</th><th scope="col" className="col-actions">操作</th></tr></thead><tbody>{filtered.map((item) => {
          const canCopy = !subscriptionActions || subscriptionAvailable(item);
          const protectedRow = protectedItem?.(item) ?? false;
          return <tr key={item.id}>{columns.map((column, index) => <td className={column.className} data-label={column.label} key={column.label}>{list && index === 0 ? <button type="button" className="clear-resource-open" aria-label={`查看并编辑 ${resourceName(item)}`} onClick={() => showEdit(item)}>{column.render(item)}</button> : column.render(item)}</td>)}<td data-label="状态"><StatusBadge status={item.status} /></td><td className="row-actions" data-label="操作"><ResourceRowActions enabled={Boolean(list)}>
            {identifierAction && <button type="button" className="icon-button" aria-label={`复制 ${item.id} 的 ${identifierAction.label}`} title={`复制 ${identifierAction.label}`} disabled={copyingIdentifier === item.id} onClick={() => void copyIdentifier(item)}><Copy size={16} /></button>}
            {subscriptionActions && <button type="button" className="icon-button" aria-label={`复制 ${item.id} 的订阅链接`} title={canCopy ? "复制订阅链接" : "完整令牌尚未保存"} disabled={!canCopy || copying === item.id} onClick={() => void copySubscription(item)}><Link2 size={16} /></button>}
			{portalActions && !list && <button type="button" className="icon-button" aria-label={`进入 ${item.id} 的订阅页面`} title="进入订阅页面" disabled={openingPortal === item.id} onClick={() => void openPortal(item)}><SquareArrowOutUpRight size={16} /></button>}
            {upgradeable && !list && <button type="button" className={`icon-button ${agentUpgradeState(item) === "available" ? "icon-button--update" : ""}`} aria-label={`升级 ${resourceName(item)} 的 Agent`} title={upgradeStateLabel(item)} disabled={!canRequestUpgrade(item) || upgrading} onClick={() => setUpgradeTarget(item)}><RefreshCw size={16} /></button>}
            <button type="button" className={list ? "clear-row-edit" : "icon-button"} aria-label={`编辑 ${list ? resourceName(item) : item.id}`} title="编辑" onClick={() => showEdit(item)}>{list ? "编辑" : <Edit3 size={16} />}</button>
            <RowMenu className={list ? "clear-resource-menu" : undefined} label={`${item.id} 更多操作`}>{upgradeable && list && <button type="button" disabled={!canRequestUpgrade(item) || upgrading} title={upgradeStateLabel(item)} onClick={() => setUpgradeTarget(item)}><RefreshCw size={15} />{upgradeStateLabel(item)}</button>}<button type="button" onClick={() => showEdit(item)}><Edit3 size={15} />编辑</button>{identifierAction && <button type="button" disabled={copyingIdentifier === item.id} onClick={() => void copyIdentifier(item)}><Copy size={15} />复制 {identifierAction.label}</button>}{subscriptionActions && <button type="button" disabled={!canCopy || copying === item.id} onClick={() => void copySubscription(item)}><Link2 size={15} />{canCopy ? "复制订阅链接" : "完整令牌未保存"}</button>}{portalActions && <button type="button" disabled={openingPortal === item.id} onClick={() => void openPortal(item)}><SquareArrowOutUpRight size={15} />进入订阅页面</button>}{installable && <button type="button" onClick={() => void loadInstallation(item)}><Terminal size={15} />一键安装 Agent</button>}{identityResettable && <button type="button" onClick={() => setIdentityResetTarget(item)}><KeyRound size={15} />重置 Agent 身份</button>}{!protectedRow && <button type="button" disabled={changingStatus === item.id} onClick={() => void toggleStatus(item)}><Power size={15} />{item.status === disabledStatus ? "恢复启用" : disabledStatus === "paused" ? "暂停" : "停用"}</button>}{deletable && !protectedRow && <button type="button" className="menu-action--danger" onClick={() => setDeleteTarget(item)}><Trash2 size={15} />删除</button>}</RowMenu>
          </ResourceRowActions></td></tr>;
        })}</tbody></table></div>
      )}
    </div>
    {list && !loading && !error && <ResourceListFooter visible={filtered.length} total={data.length} />}
    </ResourceListSurface>
    {list && <ResourceListNote>{list.note}</ResourceListNote>}
      <Drawer className={list ? `clear-resource-editor${["routes", "outbounds"].includes(list.kind) ? " clear-operation-editor" : ""}` : undefined} wide={fields.some((field) => field.type === "route-rules" || field.type === "outbound-config")} open={open} title={editing ? `编辑${title}` : createLabel} description={editing ? `修改 ${editing.id} 的配置` : `创建新的${title}记录`} onClose={requestCloseEditor}><form className="drawer-form" onSubmit={submit} aria-busy={saving} noValidate>{formError && <div className="form-error" role="alert">{formError}</div>}{fields.filter((field) => !field.editOnly || editing).map((field) => <Field group={field.type === "node-picker" || field.type === "route-rules" || field.type === "outbound-config"} key={field.key} label={field.label} required={field.required} helper={field.helper} error={errors[field.key]}>{field.type === "textarea" || field.type === "json" ? <textarea disabled={field.disabled?.(editing) ?? false} className={field.type === "json" ? "code-editor code-editor--small" : ""} rows={field.type === "json" ? 9 : 4} value={values[field.key] ?? ""} placeholder={field.placeholder} onChange={(event) => updateValue(field.key, event.target.value)} /> : field.type === "select" ? <select disabled={Boolean((editing && field.lockedValue && values[field.key] === field.lockedValue) || field.disabled?.(editing))} value={values[field.key] ?? ""} onChange={(event) => updateValue(field.key, event.target.value)}>{field.options?.filter((option) => !option.exclusive || values[field.key] === option.value).map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select> : field.type === "node-picker" ? <NodeMultiSelect value={values[field.key] ?? ""} options={field.options ?? []} onChange={(value) => updateValue(field.key, value)} /> : field.type === "route-rules" ? <RouteRulesEditor value={values[field.key] ?? "[]"} outbounds={field.routeOutbounds ?? []} onChange={(value) => updateValue(field.key, value)} /> : field.type === "outbound-config" ? <OutboundConfigForm protocol={values.protocol ?? "socks"} settings={parseOutboundSettings(values.settings ?? "{}")} proxyTag={values.proxy_tag ?? ""} outbounds={field.routeOutbounds ?? []} currentID={editing?.id} onProtocolChange={(protocol, settings) => { setValues((current) => ({ ...current, protocol, settings: JSON.stringify(settings) })); setErrors((current) => ({ ...current, [field.key]: "" })); }} onSettingsChange={(settings) => updateValue("settings", JSON.stringify(settings))} onProxyTagChange={(value) => updateValue("proxy_tag", value)} /> : <input disabled={field.disabled?.(editing) ?? false} type={field.type ?? "text"} value={values[field.key] ?? ""} placeholder={field.placeholder} onChange={(event) => updateValue(field.key, event.target.value)} />}</Field>)}<footer className="drawer__actions"><Button type="button" variant="ghost" disabled={saving} onClick={requestCloseEditor}>取消</Button><Button type="button" variant="primary" loading={saving} onClick={() => void save()}>{saving ? editing ? "保存中…" : "创建中…" : editing ? "保存修改" : "创建"}</Button></footer></form></Drawer>
    <ConfirmDialog open={discardEditor} title="放弃未保存修改？" description="当前表单中的修改尚未保存，关闭后将丢失这些内容。" confirmLabel="放弃修改" loading={saving} onClose={() => setDiscardEditor(false)} onConfirm={() => { setDiscardEditor(false); setOpen(false); }} />
    <Drawer className={list ? "clear-resource-editor" : undefined} open={Boolean(install)} title="一键安装 Corade Agent" description={install ? `目标服务器：${install.name}` : undefined} onClose={() => setInstall(null)}>{install && <div className="install-command"><p>在目标服务器的 root shell 中执行以下命令。</p><textarea className="code-editor" readOnly rows={7} value={install.command} /><footer><Button variant="primary" onClick={() => void writeClipboard(install.command).then(() => toast("安装命令已复制")).catch(() => toast("复制失败，请手动选择命令", "error"))}><Copy size={16} />复制命令</Button></footer></div>}</Drawer>
    <ConfirmDialog open={Boolean(deleteTarget)} title={`删除${subscriptionActions ? "这个账号" : `这条${title}记录`}？`} description={deleteTarget ? subscriptionActions ? `“${resourceName(deleteTarget)}”将从订阅账号列表中移除，现有订阅链接会立即失效。流量与审计历史仍会保留。` : `“${resourceName(deleteTarget)}”将从${title}列表中移除。存在关联资源时系统会阻止删除，审计历史仍会保留。` : ""} confirmLabel="确认删除" loading={deleting} onClose={() => setDeleteTarget(null)} onConfirm={() => void deleteResource()} />
    <ConfirmDialog open={Boolean(upgradeTarget)} title="升级 Agent" description={upgradeTarget ? `将在 ${resourceName(upgradeTarget)} 的下一次心跳中领取升级任务，并从 GitHub 更新至 ${latestAgentVersion(upgradeTarget)}。当前 Agent 会在独立任务中重启。` : ""} confirmLabel="下发升级任务" confirmVariant="primary" loading={upgrading} onClose={() => { if (!upgrading) setUpgradeTarget(null); }} onConfirm={() => void requestUpgrade()} />
    <ConfirmDialog open={Boolean(identityResetTarget)} title="重置 Agent 身份？" description={identityResetTarget ? `将清除 ${resourceName(identityResetTarget)} 当前绑定的 Agent 公钥和在线状态。仅在服务器重装系统或 Agent 身份文件丢失后使用；随后需要重新生成并执行一键安装命令。` : ""} confirmLabel="确认重置" loading={resettingIdentity} onClose={() => { if (!resettingIdentity) setIdentityResetTarget(null); }} onConfirm={() => void resetAgentIdentity()} />
  </div>;
}

const primary = (name: string, id: string, detail?: string) => <><strong className="cell-primary">{name}</strong><span className="resource-id">{detail || id}</span></>;
function sameFormValues(left: Record<string, string>, right: Record<string, string>) {
  const keys = new Set([...Object.keys(left), ...Object.keys(right)]);
  return [...keys].every((key) => (left[key] ?? "") === (right[key] ?? ""));
}
const subscriptionAvailable = (item: Resource) => "subscription_available" in item && item.subscription_available === true;
const resourceName = (item: Resource) => "name" in item && typeof item.name === "string" ? item.name : item.id;
const statusLabel = (status: string) => ({ active: "启用", online: "在线", published: "已发布", pending: "待接入", draft: "待发布", disabled: "已停用", paused: "已暂停", expired: "已到期", offline: "离线", error: "异常", archived: "已归档" } as Record<string, string>)[status] ?? status;
function requestErrorMessage(reason: unknown, fallback: string) {
  if (!(reason instanceof ApiError)) return reason instanceof Error ? reason.message : fallback;
  if (reason.status === 401 || reason.code === "UNAUTHORIZED") return "登录状态已失效，请重新登录";
  if (reason.status === 403 || reason.code === "FORBIDDEN") return "当前账号没有执行此操作的权限";
  if (reason.status === 409 || reason.code === "RESOURCE_CONFLICT") return reason.message || "资源仍被其他配置引用，请先解除关联";
  if (reason.status === 422 || reason.code === "VALIDATION_ERROR") return reason.message || "请检查表单中的字段";
  if (reason.status === 408 || reason.code === "REQUEST_TIMEOUT") return "请求超时，请检查网络后重试";
  return reason.message || fallback;
}
type AgentUpgradeState = "disconnected" | "pending" | "running" | "latest" | "available" | "unknown";
const latestAgentVersion = (item: Resource) => "latest_agent_version" in item && typeof item.latest_agent_version === "string" ? item.latest_agent_version : "";
const agentUpgradeState = (item: Resource): AgentUpgradeState => {
  if (!("last_heartbeat_at" in item) || !item.last_heartbeat_at) return "disconnected";
  if ("agent_upgrade_status" in item) {
    const status = item.agent_upgrade_status;
    if (status === "succeeded") return "latest";
    if (status === "failed" || status === "timed_out") {
      const latest = latestAgentVersion(item);
      return latest && "agent_version" in item && typeof item.agent_version === "string" && item.agent_version.toLowerCase() !== latest.toLowerCase() ? "available" : "unknown";
    }
    if (status === "queued") return "pending";
    if (status === "dispatched" || status === "acknowledged") return "running";
  }
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
    list={{ kind: "machines", searchPlaceholder: "搜索名称、IP 或域名", note: "资源数据随心跳更新。离线服务器保留最后上报信息，请留意采样时间。", filter: { label: "按区域筛选服务器", allLabel: "全部区域", value: (item) => item.region || "未设置" } }}
    createLabel="添加服务器"
    installable
    upgradeable
    identityResettable
    refreshIntervalMs={15_000}
    enabledStatus="pending"
    defaults={{ name: "", region: "", host: "", labels: "{}", notes: "" }}
    fields={[{ key: "name", label: "服务器名称", required: true, placeholder: "例如：香港边缘 01" }, { key: "region", label: "区域", placeholder: "例如：香港" }, { key: "host", label: "IP 或域名", placeholder: "hk01.example.net" }, { key: "labels", label: "标签 JSON", type: "json", helper: "用于筛选和自动化识别", placeholder: "{}" }, { key: "notes", label: "备注", type: "textarea" }]}
    transform={(v) => ({ name: v.name, region: v.region, host: v.host, labels: JSON.parse(v.labels || "{}"), notes: v.notes })}
    toValues={(item) => ({ name: item.name, region: item.region, host: item.host, labels: JSON.stringify(item.labels ?? {}, null, 2), notes: item.notes ?? "" })}
    columns={[
      { label: "服务器", width: "25%", render: (item) => primary(item.name, item.id, item.host || item.id) },
      { label: "区域 / 节点", width: "12%", render: (item) => <>{item.region || "未设置"}<span className="resource-id">{item.node_count} 个节点</span></> },
      { label: "资源占用", width: "17%", render: (item) => <MachineMetrics machine={item} /> },
      { label: "Agent", width: "18%", render: (item) => <><span className="cell-primary mono">{item.agent_version || "未接入"}</span><span className={`resource-id ${item.agent_protocol === "v2" ? "text-success" : ""}`}>{item.agent_protocol === "v2" ? "V2 加密" : "旧通讯"} · {agentUpgradeLabel(item)}</span></> },
      { label: "最后心跳", render: (item) => <ResourceDate value={item.last_heartbeat_at} formatted={formatPreciseDate(item.last_heartbeat_at)} /> },
    ]}
  />;
}

function MachineMetrics({ machine }: { machine: Machine }) {
  const memory = usagePercent(machine.metrics?.mem?.used, machine.metrics?.mem?.total);
  const disk = usagePercent(machine.metrics?.disk?.used, machine.metrics?.disk?.total);
  if (!machine.metrics_sampled_at) return <span className="history-muted">等待心跳</span>;
  return <div className="machine-metrics" title={`采样时间 ${formatPreciseDate(machine.metrics_sampled_at)}`}>
    {[["CPU", machine.metrics?.cpu], ["内存", memory], ["磁盘", disk]].map(([label, value]) => <span className="machine-metric" key={label}><b>{label}</b><i className="machine-metric__track" aria-hidden="true"><i style={{ width: `${typeof value === "number" && Number.isFinite(value) ? Math.max(0, Math.min(100, value)) : 0}%` }} /></i><span>{formatPercent(typeof value === "number" ? value : undefined)}</span></span>)}
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
  if (machine.agent_upgrade_status === "failed") return `上次升级失败${machine.agent_upgrade_error ? `：${machine.agent_upgrade_error}` : ""}`;
  if (machine.agent_upgrade_status === "timed_out") return "上次升级超时，可重新下发";
  if (machine.agent_upgrade_status === "succeeded") return `已升级至 ${machine.agent_version || machine.agent_upgrade_target_version || "最新版本"}`;
  if (state === "available") return `可更新至 ${machine.latest_agent_version}`;
  return ({ disconnected: "尚未连接", pending: "升级任务等待心跳", running: "升级执行中", latest: "已是最新版本", unknown: "版本检查暂不可用" } as const)[state];
}

export function AccessGroupsPage() {
  const { data: nodes } = useResource<Node[]>("/nodes", []);
  return <ResourcePage<AccessGroup> list={{ kind: "groups", searchPlaceholder: "搜索权限组名称或备注", note: "权限组定义节点访问范围，不是后台管理员角色。节点需处于已发布状态才可使用。" }} title="权限组" description="控制用户和朋友可以访问的节点范围" endpoint="/access-groups" createLabel="添加权限组" defaults={{ name: "", notes: "", node_ids: "" }} fields={[{ key: "name", label: "权限组名称", required: true, placeholder: "例如：标准线路" }, { key: "node_ids", label: "可访问节点", type: "node-picker", options: nodes.map((item) => ({ value: item.id, label: item.name, detail: `${item.protocol.toUpperCase()} · ${item.machine_name} · 端口 ${item.server_port}`, keywords: `${item.id} ${item.machine_name} ${item.protocol} ${item.server_port}` })) }, { key: "notes", label: "备注", type: "textarea", placeholder: "选填，仅管理员可见" }]} transform={(v) => ({ name: v.name, notes: v.notes, node_ids: v.node_ids.split(",").filter(Boolean) })} toValues={(item) => ({ name: item.name, notes: item.notes ?? "", node_ids: (item.node_ids ?? []).join(",") })} columns={[{ label: "权限组", width: "25%", render: (item) => primary(item.name, item.id, item.notes) }, { label: "可访问节点", width: "31%", render: (item) => <><span className="mono">{item.node_count} 个节点</span><div className="resource-node-chips">{(item.node_ids ?? []).slice(0, 3).map((id) => <span key={id}>{nodes.find((node) => node.id === id)?.name || id}</span>)}{(item.node_ids?.length ?? 0) > 3 && <span>+{item.node_ids.length - 3}</span>}</div></> }, { label: "账号数", width: "9%", render: (item) => <span className="mono">{item.user_count}</span> }, { label: "更新时间", render: (item) => <ResourceDate value={item.updated_at} formatted={formatDateWithYear(item.updated_at)} /> }]} />;
}

export function PlansPage() {
  const { data: groups } = useResource<AccessGroup[]>("/access-groups", []);
  return <ResourcePage<Plan>
    title="套餐"
    description="定义流量、重置周期、速率、设备数和默认访问范围"
    endpoint="/plans"
    list={{ kind: "plans", searchPlaceholder: "搜索套餐名称或权限组", note: "套餐由管理员手动分配。自然月按站点时区每月 1 日重置流量，不自动重置则持续累计。" }}
    createLabel="添加套餐"
    defaults={{ name: "", access_group_id: groups[0]?.id ?? "", traffic_gb: "500", speed_limit_mbps: "100", device_limit: "3", reset_strategy: "calendar_month", default_valid_days: "30", notes: "" }}
    fields={[
      { key: "name", label: "套餐名称", required: true },
      { key: "access_group_id", label: "默认权限组", type: "select", required: true, options: [{ value: "", label: "请选择权限组" }, ...groups.map((item) => ({ value: item.id, label: item.name }))] },
      { key: "traffic_gb", label: "套餐流量（GB）", type: "number", required: true },
      { key: "reset_strategy", label: "流量重置", type: "select", required: true, helper: "自然月按站点时区每月 1 日 00:00 归零；不自动重置会从当前用量继续累计", options: [{ value: "calendar_month", label: "自然月重置" }, { value: "never", label: "不自动重置（持续累计）" }] },
      { key: "speed_limit_mbps", label: "速率限制（Mbps）", type: "number" },
      { key: "device_limit", label: "同时在线设备数", type: "number" },
      { key: "default_valid_days", label: "默认有效天数", type: "number" },
      { key: "notes", label: "备注", type: "textarea" },
    ]}
    transform={(v) => ({ name: v.name, access_group_id: v.access_group_id, traffic_limit_bytes: Number(v.traffic_gb) * 1024 ** 3, speed_limit_mbps: Number(v.speed_limit_mbps || 0), device_limit: Number(v.device_limit || 0), reset_strategy: v.reset_strategy, default_valid_days: Number(v.default_valid_days || 0), notes: v.notes })}
    toValues={(item) => ({ name: item.name, access_group_id: item.access_group_id, traffic_gb: String(item.traffic_limit_bytes / 1024 ** 3), speed_limit_mbps: String(item.speed_limit_mbps), device_limit: String(item.device_limit), reset_strategy: item.reset_strategy, default_valid_days: String(item.default_valid_days), notes: item.notes ?? "" })}
    columns={[
      { label: "套餐 / 权限组", width: "25%", render: (item) => primary(item.name, item.id, item.access_group_name) },
      { label: "流量", width: "13%", className: "mono", render: (item) => formatBytes(item.traffic_limit_bytes) },
      { label: "速率", width: "13%", className: "mono", render: (item) => `${item.speed_limit_mbps || "不限"}${item.speed_limit_mbps ? " Mbps" : ""}` },
      { label: "设备", width: "8%", render: (item) => item.device_limit ? `${item.device_limit} 台` : "不限" },
      { label: "账号数", width: "9%", render: (item) => <span className="mono">{item.user_count}</span> },
      { label: "重置周期", render: (item) => <>{item.reset_strategy === "never" ? "不自动重置" : "自然月"}<span className="resource-id">{item.default_valid_days ? `${item.default_valid_days} 天有效期` : "不限制有效期"}</span></> },
    ]}
  />;
}

export function UsersPage() {
  const { data: plans } = useResource<Plan[]>("/plans", []);
  return <ResourcePage<User>
    title="订阅账号"
    description="管理员可登录后台并使用订阅；用户与朋友仅能使用订阅"
    endpoint="/users"
    list={{ kind: "users", searchPlaceholder: "搜索名称、邮箱或门户账号", note: "管理员可登录后台，用户与朋友使用订阅中心。管理员代入订阅中心时保持只读。", categories: [{ value: "user", label: "用户", matches: (item) => item.role === "user" }, { value: "friend", label: "朋友", matches: (item) => item.role === "friend" }, { value: "admin", label: "管理员", matches: (item) => item.role === "admin" }], filter: { label: "按套餐筛选订阅账号", allLabel: "全部套餐", value: (item) => item.plan_name || "未分配" } }}
    createLabel="添加用户或朋友"
    subscriptionActions
	portalActions
    quickAccountCreation
    deletable
    disabledStatus="paused"
    protectedItem={(item) => item.role === "admin"}
    defaults={{ role: "user", name: "", email: "", plan_id: plans[0]?.id ?? "", expires_at: "", notes: "", portal_login: "", portal_password: "" }}
    fields={[
      { key: "role", label: "账号分类", type: "select", lockedValue: "admin", options: [{ value: "admin", label: "管理员", exclusive: true }, { value: "user", label: "用户" }, { value: "friend", label: "朋友" }] },
      { key: "uuid", label: "UDID", editOnly: true, required: true, helper: "修改后会随节点成员配置同步到 Corade" },
      { key: "name", label: "账号名称", required: true },
      { key: "email", label: "邮箱", type: "email" },
      { key: "plan_id", label: "套餐", type: "select", options: [{ value: "", label: "暂不分配" }, ...plans.map((item) => ({ value: item.id, label: item.name }))] },
      { key: "expires_at", label: "到期时间", type: "datetime-local" },
      { key: "notes", label: "内部备注", type: "textarea" },
		{ key: "portal_login", label: "门户登录账号", helper: "用户在 /edu 使用的唯一账号。已有邮箱会自动预填。", disabled: (item) => item?.role === "admin" },
		{ key: "portal_password", label: "设置或重置门户密码", type: "password", helper: "至少 8 位；编辑时留空则不修改。管理员订阅账号使用后台一键进入。", disabled: (item) => item?.role === "admin" },
    ]}
    transform={(v, editing) => {
      const email = v.email.trim();
      const values = { role: v.role, name: v.name.trim(), email: editing ? email : email || null, plan_id: editing ? v.plan_id : v.plan_id || null, expires_at: v.expires_at ? new Date(v.expires_at).toISOString() : editing ? "" : null, notes: v.notes, portal_login: v.portal_login.trim(), portal_password: v.portal_password };
      return editing ? { ...values, uuid: v.uuid } : values;
    }}
    toValues={(item) => ({ role: item.role, uuid: item.uuid, name: item.name, email: item.email ?? "", plan_id: item.plan_id ?? "", expires_at: localDateTime(item.expires_at), notes: item.notes ?? "", portal_login: item.portal_login ?? "", portal_password: "" })}
    columns={[
      { label: "账号", width: "21%", render: (item) => primary(item.name, item.id, item.email || item.id) },
      { label: "分类", width: "8%", render: (item) => <span className="resource-role">{item.role === "admin" && <KeyRound size={12} aria-hidden="true" />}{item.role === "admin" ? "管理员" : item.role === "friend" ? "朋友" : "用户"}</span> },
      { label: "套餐", width: "11%", render: (item) => item.plan_name || "未分配" },
      { label: "流量用量", width: "16%", render: (item) => <TrafficUsage used={item.traffic_used_bytes} limit={item.traffic_limit_bytes} /> },
		{ label: "门户", width: "13%", render: (item) => item.role === "admin" ? <span>后台进入<span className="resource-id">管理员只读代入</span></span> : item.portal_enabled ? item.portal_login : "未启用" },
      { label: "到期", render: (item) => item.expires_at ? <ResourceDate value={item.expires_at} formatted={formatDateWithYear(item.expires_at)} /> : "长期" },
    ]}
  />;
}

export function RoutesPage() {
  const { data: outbounds } = useResource<Outbound[]>("/outbounds", []);
  const initialRules = JSON.stringify([]);
  const defaultOutboundOptions = [{ value: "", label: "默认直连（仅按下方规则处理）" }, ...outbounds.filter((item) => item.status === "active" && !["direct", "block"].includes(item.protocol) && item.kernel_support.includes("xray")).map((item) => ({ value: item.tag, label: `${item.name} · ${item.tag}` }))];
  return <ResourcePage<RoutePolicy> title="路由策略" description="按顺序匹配域名、IP、端口与网络协议，并执行直连、阻断或指定出站" endpoint="/route-policies" list={{ kind: "routes", searchPlaceholder: "搜索策略名称、备注或 ID", note: "节点分别绑定默认、管理员、用户与朋友策略；未单独绑定时回退默认策略。规则从上到下匹配。", categories: [{ value: "default", label: "默认", matches: (item) => !item.scope || item.scope === "default" }, { value: "admin", label: "管理员", matches: (item) => item.scope === "admin" }, { value: "member", label: "用户与朋友", matches: (item) => item.scope === "member" }] }} createLabel="添加路由策略" defaults={{ name: "", scope: "default", notes: "", default_outbound_tag: "", rules: initialRules }} fields={[{ key: "name", label: "策略名称", required: true, placeholder: "例如：流媒体直连" }, { key: "scope", label: "适用范围", type: "select", options: [{ value: "default", label: "默认策略" }, { value: "admin", label: "管理员订阅" }, { value: "member", label: "用户与朋友订阅" }], helper: "节点可分别绑定管理员策略和用户/朋友策略；未绑定时回退默认策略。" }, { key: "default_outbound_tag", label: "默认流量出口", type: "select", options: defaultOutboundOptions, helper: "选择出口后，未被规则命中的流量将通过该出口；未选择时保持直连。" }, { key: "notes", label: "策略备注", type: "textarea", placeholder: "选填，例如适用区域或变更原因" }, { key: "rules", label: "规则编排", type: "route-rules", routeOutbounds: outbounds }]} transform={(v) => ({ name: v.name.trim(), scope: v.scope, notes: v.notes.trim(), default_outbound_tag: v.default_outbound_tag, rules: JSON.parse(v.rules || "[]") })} toValues={(item) => ({ name: item.name, scope: item.scope ?? "default", notes: item.notes ?? "", default_outbound_tag: item.default_outbound_tag ?? "", rules: JSON.stringify(item.rules ?? []) })} columns={[{ label: "策略", render: (item) => primary(item.name, item.id, item.notes) }, { label: "适用范围", render: (item) => item.scope === "admin" ? "管理员" : item.scope === "member" ? "用户与朋友" : "默认" }, { label: "默认出口", render: (item) => item.default_outbound_tag || "直连" }, { label: "规则", render: (item) => <span className="mono">{item.rules?.length ?? 0}</span> }, { label: "当前版本", render: (item) => <span className="mono">rev {item.current_revision}</span> }, { label: "绑定节点", render: (item) => <span className="mono">{item.node_count}</span> }, { label: "更新时间", render: (item) => <ResourceDate value={item.updated_at} formatted={formatDateWithYear(item.updated_at)} /> }]} />;
}

export function OutboundsPage() {
  const { data: outbounds } = useResource<Outbound[]>("/outbounds", []);
  return <ResourcePage<Outbound> title="出站" description="维护路由策略可选择的 Xray 代理出站" endpoint="/outbounds" list={{ kind: "outbounds", searchPlaceholder: "搜索名称、标记或服务器地址", note: "统一维护可供路由选择的 Xray 出站。上游用于链式代理；当前不提供出站连通性探测。", categories: ["socks", "vless", "vmess", "trojan", "shadowsocks", "http", "wireguard"].map((protocol) => ({ value: protocol, label: protocol === "shadowsocks" ? "SS" : protocol.toUpperCase(), matches: (item) => item.protocol === protocol })) }} createLabel="添加出站" defaults={{ name: "", tag: "", protocol: "socks", proxy_tag: "", settings: JSON.stringify(defaultOutboundSettings("socks")), kernel_support: "xray" }} fields={[{ key: "name", label: "出站名称", required: true, placeholder: "例如：新加坡 SOCKS 出站" }, { key: "tag", label: "唯一标记", required: true, placeholder: "sg-socks" }, { key: "settings", label: "协议与连接参数", type: "outbound-config", required: true, routeOutbounds: outbounds }]} transform={(v) => ({ name: v.name.trim(), tag: v.tag.trim(), protocol: v.protocol, proxy_tag: v.proxy_tag, settings: JSON.parse(v.settings || "{}"), kernel_support: ["xray"] })} toValues={(item) => ({ name: item.name, tag: item.tag, protocol: item.protocol, proxy_tag: item.proxy_tag, settings: JSON.stringify(item.settings ?? {}), kernel_support: (item.kernel_support ?? ["xray"]).join(",") })} columns={[{ label: "出站", render: (item) => primary(item.name, item.id, item.tag) }, { label: "协议", render: (item) => <span className="protocol-label">{item.protocol}</span> }, { label: "服务器", render: (item) => <span className="mono">{outboundAddress(item.settings)}</span> }, { label: "上游", render: (item) => <>{item.proxy_tag || "无"}<span className="resource-id">{item.proxy_tag ? "通过上游出站连接" : "直接连接目标"}</span></> }, { label: "内核", render: (item) => item.kernel_support.map((kernel) => kernel === "xray" ? "Xray" : kernel).join(" / ") }, { label: "更新时间", render: (item) => <ResourceDate value={item.updated_at} formatted={formatDateWithYear(item.updated_at)} /> }]} />;
}

function outboundAddress(settings: Record<string, unknown>) {
  const servers = settings.servers ?? settings.vnext;
  const server = Array.isArray(servers) ? servers[0] as Record<string, unknown> | undefined : undefined;
  if (server?.address) return `${server.address}:${server.port ?? "—"}`;
  const peers = settings.peers;
  const peer = Array.isArray(peers) ? peers[0] as Record<string, unknown> | undefined : undefined;
  return String(peer?.endpoint || "—");
}
