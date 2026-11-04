import { useMemo, useRef, useState } from "react";
import { Link, useLocation, useSearchParams } from "wouter";
import { Copy, Edit3, Eye, EyeOff, Filter, LayoutGrid, List, KeyRound, Link2, Plus, Power, RefreshCw, Search, SquareArrowOutUpRight, Terminal, Trash2, UserPlus, UsersRound } from "lucide-react";
import { ApiError, api } from "../api";
import { Button, ConfirmDialog, DataFreshness, DetailDialog, EmptyState, PageHeader, RowMenu, StatusBadge, TableSkeleton, formatPreciseDate, useToast } from "../components/ui";
import { ResourceCategoryFilters, ResourceListFooter, ResourceListNote, ResourceListSurface, ResourceRowActions } from "../components/ResourceList";
import { useResource } from "../hooks";
import { editorHref, useRestoreList } from "../navigation";
import { subscriptionAvailable, resourceName, statusLabel, requestErrorMessage, agentUpgradeState, latestAgentVersion, canRequestUpgrade, upgradeStateLabel, writeClipboard } from "../resourceModel";
import { MachineMetrics } from "../components/MachineMetrics";
import type { Machine } from "../types";
import type { Resource, ResourceDefinition } from "../resourceDefinitions";
import { creationIntent } from "../mutations";

export function ResourceListPage<T extends Resource>({ title, description, endpoint, createLabel, columns, enabledStatus = "active", disabledStatus = "disabled", installable = false, upgradeable = false, identityResettable = false, subscriptionActions = false, portalActions = false, quickAccountCreation = false, deletable = false, identifierAction, protectedItem, refreshIntervalMs = 0, list }: ResourceDefinition<T>) {
	deletable = deletable || ["/machines", "/access-groups", "/plans", "/route-policies", "/outbounds"].includes(endpoint);
	if (endpoint === "/route-policies") enabledStatus = "published";
  const { data, loading, refreshing, error, refreshError, lastSuccessAt, stale, reload } = useResource<T[]>(endpoint, [], refreshIntervalMs);
  const [params, setParams] = useSearchParams();
  const categoryKey = list?.kind === "users" ? "role" : list?.kind === "routes" ? "scope" : list?.kind === "outbounds" ? "protocol" : "status";
  const detailKey = list?.kind === "machines" ? "region" : list?.kind === "users" ? "plan" : "filter";
  const query = params.get("q") ?? "";
  const statusFilter = list?.categories ? params.get("status") || "all" : "all";
  const category = params.get(categoryKey) || "all";
  const detailFilter = params.get(detailKey) || "";
  function updateFilters(values: Record<string, string>) {
    setParams((current) => {
      const next = new URLSearchParams(current);
      for (const [key, value] of Object.entries(values)) {
        if (!value || value === "all") next.delete(key); else next.set(key, value);
      }
      next.delete("page");
      return next;
    }, { replace: true });
  }
  const setQuery = (value: string) => updateFilters({ q: value });
  const setStatusFilter = (value: string) => updateFilters({ status: value });
  const setCategory = (value: string) => updateFilters({ [categoryKey]: value });
  const setDetailFilter = (value: string) => updateFilters({ [detailKey]: value });
  const machines = endpoint === "/machines";
  const view = params.get("view") === "table" ? "table" : "cards";
  const hideAddress = params.get("hide_address") === "1";
  const locks = useRef(new Set<string>());
  const quickIntents = useRef({ user: creationIntent(), friend: creationIntent() });
  const [installing, setInstalling] = useState("");
  const [statusTarget, setStatusTarget] = useState<T | null>(null);
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
  const base = endpoint === "/route-policies" ? "/routes" : endpoint;
  const [, navigate] = useLocation();
  useRestoreList(!loading && !error);
  function showCreate() { navigate(editorHref(base)); }
  function showEdit(item: T) { navigate(editorHref(base, item.id)); }

  async function toggleStatus(item: T) {
    if (locks.current.has("status")) return;
    locks.current.add("status");
    const nextStatus = item.status === disabledStatus ? enabledStatus : disabledStatus;
    setChangingStatus(item.id);
    try {
      await api.patch(`${endpoint}/${item.id}`, { status: nextStatus });
      toast(nextStatus === disabledStatus ? `${title}已${disabledStatus === "paused" ? "暂停" : "停用"}` : `${title}已恢复`);
      setStatusTarget(null);
      await reload();
    } catch (reason) {
      toast(reason instanceof ApiError ? reason : requestErrorMessage(reason, "状态修改失败"), "error");
    } finally {
      locks.current.delete("status");
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
      toast(reason instanceof ApiError ? reason : requestErrorMessage(reason, "订阅链接复制失败，请重试"), "error");
    } finally {
      setCopying("");
    }
  }

	async function openPortal(item: T) {
		const popup = window.open("about:blank", "_blank");
		if (!popup) { toast("浏览器阻止了新窗口，请允许本站弹出窗口后重试。", "error"); return; }
		popup.opener = null;
		popup.document.title = "正在打开订阅中心";
		popup.document.body.textContent = "正在获取只读访问授权，请稍候…";
		setOpeningPortal(item.id);
		try {
			const result = await api.post<{ grant: string }>(`${endpoint}/${item.id}/portal`);
			if (popup.closed) throw new Error("订阅窗口已关闭，请重新打开。");
			popup.location.replace(`/edu?grant=${encodeURIComponent(result.grant)}`);
		} catch (reason) {
			popup.close();
			toast(reason instanceof ApiError ? reason : requestErrorMessage(reason, "订阅页面打开失败，请重试"), "error");
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
    if (!deleteTarget || locks.current.has("delete")) return;
    locks.current.add("delete");
    setDeleting(true);
    try {
      await api.delete(`${endpoint}/${deleteTarget.id}`);
      toast(subscriptionActions ? "账号已删除，原订阅链接已失效" : `${resourceName(deleteTarget)} 已删除`);
      setDeleteTarget(null);
      await reload();
    } catch (reason) {
      toast(requestErrorMessage(reason, `${title}删除失败，请重试`), "error");
    } finally {
      locks.current.delete("delete");
      setDeleting(false);
    }
  }

  async function loadInstallation(item: T) {
    if (locks.current.has("install")) return;
    locks.current.add("install"); setInstalling(item.id);
    try {
      const result = await api.post<{ command: string }>(`${endpoint}/${item.id}/installation`);
      setInstall({ name: "name" in item ? String(item.name) : item.id, command: result.command });
      toast("安装凭据仅限本机，30 分钟内有效且只能登记一次；重新生成会撤销旧命令");
    } catch (reason) {
      toast(reason instanceof ApiError ? reason : requestErrorMessage(reason, "安装命令生成失败"), "error");
    } finally { locks.current.delete("install"); setInstalling(""); }
  }

  async function requestUpgrade() {
    if (!upgradeTarget || locks.current.has("upgrade")) return;
    locks.current.add("upgrade");
    setUpgrading(true);
    try {
      await api.post(`${endpoint}/${upgradeTarget.id}/agent-upgrade`);
      toast("升级任务已创建，等待下发与 Agent 确认");
      setUpgradeTarget(null);
      await reload();
    } catch (reason) {
      const message = reason instanceof ApiError ? reason.message : "升级任务下发失败，请重试";
      toast(message, "error");
    } finally {
      locks.current.delete("upgrade");
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
      toast(reason instanceof ApiError ? reason : requestErrorMessage(reason, "Agent 身份重置失败，请重试"), "error");
    } finally {
      setResettingIdentity(false);
    }
  }

  async function quickCreateAccount(role: "user" | "friend") {
    if (locks.current.has("quick")) return;
    locks.current.add("quick");
    setQuickCreating(role);
    try {
      await api.post(`${endpoint}/quick/${role}`, undefined, { idempotencyKey: quickIntents.current[role]({ role }) });
      quickIntents.current[role] = creationIntent();
      toast(`${role === "friend" ? "朋友" : "用户"}已添加，可在列表中编辑资料`);
      await reload();
    } catch (reason) {
      toast(reason instanceof ApiError ? reason : requestErrorMessage(reason, "一键添加失败，请重试"), "error");
    } finally {
      locks.current.delete("quick");
      setQuickCreating("");
    }
  }

  function renderActions(item: T) {
    const canCopy = !subscriptionActions || subscriptionAvailable(item);
    const protectedRow = protectedItem?.(item) ?? false;
    return <ResourceRowActions enabled={Boolean(list)}>
            {identifierAction && <button type="button" className="icon-button" aria-label={`复制 ${item.id} 的 ${identifierAction.label}`} title={`复制 ${identifierAction.label}`} disabled={copyingIdentifier === item.id} onClick={() => void copyIdentifier(item)}><Copy size={16} /></button>}
            {subscriptionActions && <button type="button" className="icon-button" aria-label={`复制 ${item.id} 的订阅链接`} title={canCopy ? "复制订阅链接" : "完整令牌尚未保存"} disabled={!canCopy || copying === item.id} onClick={() => void copySubscription(item)}><Link2 size={16} /></button>}
			{portalActions && !list && <button type="button" className="icon-button" aria-label={`进入 ${item.id} 的订阅页面`} title="进入订阅页面" disabled={openingPortal === item.id} onClick={() => void openPortal(item)}><SquareArrowOutUpRight size={16} /></button>}
            {upgradeable && !list && <button type="button" className={`icon-button ${agentUpgradeState(item) === "available" ? "icon-button--update" : ""}`} aria-label={`升级 ${resourceName(item)} 的 Agent`} title={upgradeStateLabel(item)} disabled={!canRequestUpgrade(item) || upgrading} onClick={() => setUpgradeTarget(item)}><RefreshCw size={16} /></button>}
            <button type="button" className={list ? "clear-row-edit" : "icon-button"} aria-label={`编辑 ${list ? resourceName(item) : item.id}`} title="编辑" onClick={() => showEdit(item)}>{list ? "编辑" : <Edit3 size={16} />}</button>
            <RowMenu className={list ? "resource-menu" : undefined} label={`${item.id} 更多操作`}>{upgradeable && list && <button type="button" disabled={!canRequestUpgrade(item) || upgrading} title={upgradeStateLabel(item)} onClick={() => setUpgradeTarget(item)}><RefreshCw size={15} />{upgradeStateLabel(item)}</button>}<button type="button" onClick={() => showEdit(item)}><Edit3 size={15} />编辑</button>{identifierAction && <button type="button" disabled={copyingIdentifier === item.id} onClick={() => void copyIdentifier(item)}><Copy size={15} />复制 {identifierAction.label}</button>}{subscriptionActions && <button type="button" disabled={!canCopy || copying === item.id} onClick={() => void copySubscription(item)}><Link2 size={15} />{canCopy ? "复制订阅链接" : "完整令牌未保存"}</button>}{portalActions && <button type="button" disabled={openingPortal === item.id} onClick={() => void openPortal(item)}><SquareArrowOutUpRight size={15} />进入订阅页面</button>}{machines && <Link to={`/nodes?machine=${encodeURIComponent(item.id)}`}><Link2 size={15} />查看关联节点</Link>}{machines && <button type="button" onClick={() => void writeClipboard((item as unknown as Machine).host).then(() => toast("服务器地址已复制")).catch(() => toast("地址复制失败", "error"))}><Copy size={15} />复制服务器地址</button>}{installable && <button type="button" disabled={Boolean(installing)} onClick={() => void loadInstallation(item)}><Terminal size={15} />{installing === item.id ? "正在生成安装命令…" : "一键安装 Agent"}</button>}{identityResettable && <button type="button" onClick={() => setIdentityResetTarget(item)}><KeyRound size={15} />重置 Agent 身份</button>}{!protectedRow && <button type="button" disabled={changingStatus === item.id} onClick={() => setStatusTarget(item)}><Power size={15} />{item.status === disabledStatus ? "恢复启用" : disabledStatus === "paused" ? "暂停" : "停用"}</button>}{deletable && !protectedRow && <button type="button" className="menu-action--danger" onClick={() => setDeleteTarget(item)}><Trash2 size={15} />删除</button>}</RowMenu>
          </ResourceRowActions>;
  }

  return <div className={`page${list ? ` resource-page resource-page--${list.kind}` : ""}`}>
    <PageHeader title={title} description={description} actions={<>{quickAccountCreation && <><Button type="button" loading={quickCreating === "user"} disabled={Boolean(quickCreating)} onClick={() => void quickCreateAccount("user")}><UserPlus size={16} />{quickCreating === "user" ? "添加中…" : "一键添加用户"}</Button><Button type="button" loading={quickCreating === "friend"} disabled={Boolean(quickCreating)} onClick={() => void quickCreateAccount("friend")}><UsersRound size={16} />{quickCreating === "friend" ? "添加中…" : "一键添加朋友"}</Button></>}<Button type="button" variant="primary" disabled={Boolean(quickCreating)} onClick={showCreate}><Plus size={16} />{createLabel}</Button></>} />
    <ResourceListSurface enabled={Boolean(list)}>
    {list && <ResourceCategoryFilters label={`${title}分类筛选`} value={category} options={[{ value: "all", label: "全部", count: data.length }, ...categories.map((option) => ({ value: option.value, label: option.label, count: data.filter(option.matches).length }))]} onChange={setCategory} />}
    <div className="toolbar"><label className="search-box"><Search size={16} aria-hidden="true" /><span className="sr-only">搜索{title}</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={list?.searchPlaceholder ?? `搜索${title}`} /></label>{statusOptions.length > 1 && (!list || list.categories) && <div className="toolbar__filters"><Filter size={15} aria-hidden="true" /><label className="sr-only" htmlFor={`${endpoint.slice(1)}-status-filter`}>按状态筛选{title}</label><select id={`${endpoint.slice(1)}-status-filter`} value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}><option value="all">全部状态</option>{statusOptions.map((status) => <option value={status} key={status}>{statusLabel(status)}</option>)}</select></div>}{list?.filter && <label className="toolbar__filters"><span className="sr-only">{list.filter.label}</span><select value={detailFilter} onChange={(event) => setDetailFilter(event.target.value)}><option value="">{list.filter.allLabel}</option>{detailOptions.map((option) => <option value={option} key={option}>{option}</option>)}</select></label>}{refreshIntervalMs > 0 && <DataFreshness lastSuccessAt={lastSuccessAt} stale={stale} refreshing={refreshing} error={refreshError} />}{machines && <div className="machine-view-tools"><div className="view-switch" role="group" aria-label="服务器显示方式"><button type="button" aria-pressed={view === "cards"} aria-label="卡片视图" onClick={() => updateFilters({ view: "" })}><LayoutGrid size={16} /></button><button type="button" aria-pressed={view === "table"} aria-label="表格视图" onClick={() => updateFilters({ view: "table" })}><List size={16} /></button></div><button type="button" className="button button--ghost" aria-pressed={hideAddress} onClick={() => updateFilters({ hide_address: hideAddress ? "" : "1" })}>{hideAddress ? <EyeOff size={16} /> : <Eye size={16} />}{hideAddress ? "显示地址" : "隐藏地址"}</button></div>}<span className="toolbar__count">{filtered.length}{filtered.length !== data.length ? ` / ${data.length}` : ""} 条记录</span></div>
    <div className="table-surface">
      {loading ? <TableSkeleton columns={columns.length + 2} /> : error ? <EmptyState title={`${title}加载失败`} description={error} action={<Button onClick={() => void reload()}>重新加载</Button>} /> : filtered.length === 0 ? <EmptyState title={data.length ? "没有符合搜索条件的记录" : `还没有${title}`} description={data.length ? "更换搜索词或调整筛选条件后重试。" : `创建第一条${title}记录开始管理。`} action={data.length && list ? <Button onClick={() => { updateFilters({ q: "", status: "", [categoryKey]: "", [detailKey]: "" }); }}>清除筛选</Button> : !data.length && <Button variant="primary" onClick={showCreate}><Plus size={16} />{createLabel}</Button>} /> : machines && view === "cards" ? <div className="machine-grid">{filtered.map((item) => { const machine = item as unknown as Machine; return <article className="machine-card" data-resource-id={item.id} key={item.id}><header><button type="button" className="machine-card__name" onClick={() => showEdit(item)}>{machine.name}</button><StatusBadge status={item.status} /></header><p className="machine-card__address mono">{hideAddress ? "地址已隐藏" : machine.host || "未设置地址"}</p><p className="machine-card__region">{machine.region || "未设置区域"}</p><MachineMetrics machine={machine} /><div className="machine-card__agent"><span>Agent <span className="mono">{machine.agent_version || "未接入"}</span> · {machine.agent_protocol === "v2" ? "V2 加密" : "旧通讯"}</span><small>{upgradeStateLabel(item)}</small></div><dl className="machine-card__timestamps"><div><dt>最后心跳</dt><dd>{formatPreciseDate(machine.last_heartbeat_at)}</dd></div><div><dt>指标采样</dt><dd>{formatPreciseDate(machine.metrics_sampled_at)}</dd></div></dl><footer><Link to={`/nodes?machine=${encodeURIComponent(item.id)}`}>{machine.node_count} 个节点</Link>{renderActions(item)}</footer></article>; })}</div> : (
        <div className="table-scroll resource-table-scroll"><table aria-label={`${title}列表`}>{list && <colgroup>{columns.map((column) => <col key={column.label} style={{ width: column.width }} />)}<col className="clear-status-col" /><col className="clear-actions-col" /></colgroup>}<thead><tr>{columns.map((column) => <th scope="col" className={column.className} key={column.label}>{column.label}</th>)}<th scope="col">状态</th><th scope="col" className="col-actions">操作</th></tr></thead><tbody>{filtered.map((item) => {
          return <tr key={item.id} data-resource-id={item.id}>{columns.map((column, index) => <td className={column.className} data-label={column.label} key={column.label}>{list && index === 0 ? <button type="button" className="clear-resource-open" aria-label={`查看并编辑 ${resourceName(item)}`} onClick={() => showEdit(item)}>{machines ? <><strong className="cell-primary">{resourceName(item)}</strong><span className="resource-id mono">{hideAddress ? "地址已隐藏" : (item as unknown as Machine).host || "未设置地址"}</span></> : column.render(item)}</button> : column.render(item)}</td>)}<td data-label="状态"><StatusBadge status={item.status} /></td><td className="row-actions" data-label="操作">{renderActions(item)}</td></tr>;
        })}</tbody></table></div>
      )}
    </div>
    {list && !loading && !error && <ResourceListFooter visible={filtered.length} total={data.length} />}
    </ResourceListSurface>
    {list && <ResourceListNote>{list.note}</ResourceListNote>}
    <DetailDialog command className={list ? "resource-detail" : undefined} open={Boolean(install)} title="一键安装 Corade Agent" description={install ? `目标服务器：${install.name}` : undefined} onClose={() => setInstall(null)}>{install && <div className="install-command"><p>在目标服务器的 root shell 中执行以下命令。</p><textarea className="code-editor" readOnly rows={7} value={install.command} /><footer><Button variant="primary" onClick={() => void writeClipboard(install.command).then(() => toast("安装命令已复制")).catch(() => toast("复制失败，请手动选择命令", "error"))}><Copy size={16} />复制命令</Button></footer></div>}</DetailDialog>
    <ConfirmDialog open={Boolean(statusTarget)} title={statusTarget?.status === disabledStatus ? "恢复这个资源？" : "停用这个资源？"} description="此操作会影响资源的可用状态，相关限制由服务端检查。" confirmLabel={statusTarget?.status === disabledStatus ? "确认恢复" : "确认停用"} loading={Boolean(changingStatus)} onClose={() => setStatusTarget(null)} onConfirm={() => { if (statusTarget) void toggleStatus(statusTarget); }} />
    <ConfirmDialog open={Boolean(deleteTarget)} title={`删除${subscriptionActions ? "这个账号" : `这条${title}记录`}？`} description={deleteTarget ? subscriptionActions ? `“${resourceName(deleteTarget)}”将从订阅账号列表中移除，现有订阅链接会立即失效。流量与审计历史仍会保留。` : `“${resourceName(deleteTarget)}”将从${title}列表中移除。存在关联资源时系统会阻止删除，审计历史仍会保留。` : ""} confirmLabel="确认删除" loading={deleting} onClose={() => setDeleteTarget(null)} onConfirm={() => void deleteResource()} />
    <ConfirmDialog open={Boolean(upgradeTarget)} title="升级 Agent" description={upgradeTarget ? `将在 ${resourceName(upgradeTarget)} 的下一次心跳中领取升级任务，并从 GitHub 更新至 ${latestAgentVersion(upgradeTarget)}。当前 Agent 会在独立任务中重启。` : ""} confirmLabel="下发升级任务" confirmVariant="primary" loading={upgrading} onClose={() => { if (!upgrading) setUpgradeTarget(null); }} onConfirm={() => void requestUpgrade()} />
    <ConfirmDialog open={Boolean(identityResetTarget)} title="重置 Agent 身份？" description={identityResetTarget ? `将清除 ${resourceName(identityResetTarget)} 当前绑定的 Agent 公钥和在线状态。仅在服务器重装系统或 Agent 身份文件丢失后使用；随后需要重新生成并执行一键安装命令。` : ""} confirmLabel="确认重置" loading={resettingIdentity} onClose={() => { if (!resettingIdentity) setIdentityResetTarget(null); }} onConfirm={() => void resetAgentIdentity()} />
  </div>;
}
