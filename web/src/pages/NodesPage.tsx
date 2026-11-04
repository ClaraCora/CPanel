import { useEffect, useMemo, useRef, useState } from "react";
import {
  Copy,
  Edit3,
  Filter,
  Plus,
  Power,
  Rocket,
  Search,
  RefreshCw,
  Trash2,
} from "lucide-react";
import { Link, useSearchParams } from "wouter";
import { ApiError, api } from "../api";
import {
  Button,
  ConfirmDialog,
  DataFreshness,
  EmptyState,
  PageHeader,
  RowMenu,
  StatusBadge,
  TableSkeleton,
  formatPreciseDate,
  useToast,
} from "../components/ui";
import { ResourceCategoryFilters, ResourceDate, ResourceListFooter, ResourceListNote, ResourceListSurface } from "../components/ResourceList";
import { useResource } from "../hooks";
import { editorHref, useRestoreList } from "../navigation";
import type { Machine, Node } from "../types";
import { creationIntent } from "../mutations";

export function NodesPage() {
  const [refreshInterval, setRefreshInterval] = useState(15_000);
  const {
    data: nodes,
    loading,
    error,
    reload,
    refreshing, refreshError, lastSuccessAt, stale,
  } = useResource<Node[]>("/nodes", [], refreshInterval);
  useEffect(() => {
    setRefreshInterval(nodes.some((node) => node.status === "published" && node.applied_revision !== node.current_revision && node.apply_status !== "failed") ? 5_000 : 15_000);
  }, [nodes]);
  const { data: machines } = useResource<Machine[]>("/machines", []);
  const [params, setParams] = useSearchParams();
  const search = params.get("q") ?? "";
  const machine = params.get("machine") ?? "";
  const status = params.get("status") ?? "";
  const locks = useRef(new Set<string>());
  const publishIntents = useRef(new Map<string, ReturnType<typeof creationIntent>>());
  const [statusTarget, setStatusTarget] = useState<Node | null>(null);
  useRestoreList(!loading && !error);
  const [publishing, setPublishing] = useState("");
  const [changingStatus, setChangingStatus] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<Node | null>(null);
  const [deleting, setDeleting] = useState(false);
  const toast = useToast();

  const filtered = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return nodes.filter(
      (node) =>
        (!needle ||
          `${node.name} ${node.machine_name} ${node.protocol} ${node.server_port}`
            .toLowerCase()
            .includes(needle)) &&
        (!machine || node.machine_id === machine) &&
        (!status || (status === "draft" ? node.status === "draft" || node.has_draft : node.status === status)),
    );
  }, [nodes, search, machine, status]);

  function syncFilters(next: {
    search?: string;
    machine?: string;
    status?: string;
  }) {
    const values = { search, machine, status, ...next };
    const query: Record<string, string> = Object.fromEntries(params);
    delete query.q; delete query.machine; delete query.status;
    if (values.search) query.q = values.search;
    if (values.machine) query.machine = values.machine;
    if (values.status) query.status = values.status;
    setParams(query, { replace: true });
  }

  async function publish(node: Node) {
    if (locks.current.has(node.id)) return;
    locks.current.add(node.id);
    setPublishing(node.id);
    try {
      if (!publishIntents.current.has(node.id)) publishIntents.current.set(node.id, creationIntent());
      await api.post(`/nodes/${node.id}/publish`, undefined, { idempotencyKey: publishIntents.current.get(node.id)!({ id: node.id, config_version: node.config_version, current_revision: node.current_revision }), configVersion: node.config_version });
      toast(`${node.name} 配置已提交，等待 Agent 应用`);
      await reload();
    } catch (reason) {
      toast(
        reason instanceof ApiError ? reason : "发布失败，请重试",
        "error",
      );
    } finally {
      locks.current.delete(node.id);
      setPublishing("");
    }
  }

  async function toggleStatus(node: Node) {
    if (locks.current.has(node.id)) return;
    locks.current.add(node.id);
    setChangingStatus(node.id);
    const nextStatus = node.status === "disabled" ? "draft" : "disabled";
    try {
      await api.patch(`/nodes/${node.id}`, { status: nextStatus });
      toast(nextStatus === "disabled" ? `${node.name} 已停用` : `${node.name} 已恢复为草稿`);
      setStatusTarget(null);
      await reload();
    } catch (reason) {
      toast(reason instanceof ApiError ? reason : "节点状态修改失败", "error");
    } finally {
      locks.current.delete(node.id);
      setChangingStatus("");
    }
  }

  async function deleteNode() {
    if (!deleteTarget || locks.current.has("delete")) return;
    locks.current.add("delete");
    setDeleting(true);
    try {
      await api.delete(`/nodes/${deleteTarget.id}`);
      toast(`${deleteTarget.name} 已删除`);
      setDeleteTarget(null);
      await reload();
    } catch (reason) {
      toast(reason instanceof ApiError ? reason : "节点删除失败，请重试", "error");
    } finally {
      locks.current.delete("delete");
      setDeleting(false);
    }
  }

  return (
    <div className="page resource-page resource-page--nodes">
      <PageHeader
        title="节点"
        description="维护 Corade 运行的入站服务配置"
        actions={<><DataFreshness lastSuccessAt={lastSuccessAt} stale={stale} refreshing={refreshing} error={refreshError} /><Button disabled={loading || refreshing} onClick={() => void reload(true)}><RefreshCw size={16} />刷新</Button><Link className="button button--primary" to={editorHref("/nodes")}><Plus size={16} />添加节点</Link></>}
      />
      <ResourceListSurface>
        <ResourceCategoryFilters label="节点分类筛选" value={status} options={[
          { value: "", label: "全部", count: nodes.length },
          ...[{ value: "published", label: "已发布" }, { value: "draft", label: "待发布" }, { value: "disabled", label: "已停用" }, { value: "error", label: "异常" }].map((option) => ({ ...option, count: nodes.filter((node) => option.value === "draft" ? node.status === "draft" || node.has_draft : node.status === option.value).length })),
        ]} onChange={(value) => { syncFilters({ status: value }); }} />
        <div className="toolbar">
          <label className="search-box"><Search size={16} aria-hidden="true" /><span className="sr-only">搜索节点</span><input value={search} onChange={(event) => { syncFilters({ search: event.target.value }); }} placeholder="搜索名称、服务器、协议或端口" /></label>
          <div className="toolbar__filters"><Filter size={15} aria-hidden="true" /><select aria-label="按服务器筛选" value={machine} onChange={(event) => { syncFilters({ machine: event.target.value }); }}><option value="">全部服务器</option>{machines.map((item) => <option value={item.id} key={item.id}>{item.name}</option>)}</select></div>
          <span className="toolbar__count">{filtered.length} 个节点</span>
        </div>
        <div className="table-surface">
          {loading ? <TableSkeleton columns={7} /> : error ? <EmptyState title="节点加载失败" description={error} action={<Button onClick={() => void reload()}>重新加载</Button>} /> : filtered.length === 0 ? <EmptyState title={nodes.length ? "没有符合筛选条件的节点" : "还没有节点"} description={nodes.length ? "调整搜索词或筛选条件后重试。" : "添加第一个节点并绑定到已经接入的服务器。"} action={nodes.length ? <Button onClick={() => { syncFilters({ search: "", machine: "", status: "" }); }}>清除筛选</Button> : <Link className="button button--primary" to={editorHref("/nodes")}><Plus size={16} />添加节点</Link>} /> : (
            <div className="table-scroll resource-table-scroll"><table aria-label="节点列表">
              <colgroup><col style={{ width: "23%" }} /><col style={{ width: "14%" }} /><col style={{ width: "16%" }} /><col style={{ width: "12%" }} /><col style={{ width: "10%" }} /><col /><col style={{ width: "14%" }} /></colgroup>
              <thead><tr><th scope="col">节点</th><th scope="col">服务器</th><th scope="col">协议 / 监听</th><th scope="col">配置版本</th><th scope="col">状态</th><th scope="col">最后上报</th><th scope="col" className="col-actions">操作</th></tr></thead>
              <tbody>{filtered.map((node) => <tr key={node.id} data-resource-id={node.id}>
                <td data-label="节点"><Link className="resource-name" to={editorHref("/nodes", node.id)}>{node.name}</Link><span className="resource-id">#{node.agent_id} · {node.id}</span></td>
                <td data-label="服务器"><Link className="cell-primary" to={editorHref("/machines", node.machine_id)}>{node.machine_name}</Link></td>
                <td data-label="协议 / 监听"><span className="protocol-label">{node.protocol}</span><span className="resource-id mono">{node.listen_ip}:{node.server_port}</span></td>
                <td data-label="配置版本"><span className="version-pair"><span>已应用 <b className="mono">{node.applied_revision}</b></span><span>当前 <b className="mono">{node.current_revision}</b></span></span>{node.has_draft && <span className="resource-id text-warning">有未发布修改</span>}{node.status === "published" && node.applied_revision !== node.current_revision && <span className={`resource-id ${node.apply_status === "failed" ? "text-danger" : "text-warning"}`}>{node.apply_status === "failed" ? `版本 ${node.attempted_revision ?? node.current_revision} 应用失败` : node.received_revision === node.current_revision ? "Agent 已接收，等待应用" : "等待 Agent 接收并应用"}</span>}<span className="resource-id">{node.kernel_type === "singbox" ? "sing-box" : "Xray"}</span></td>
                <td data-label="状态"><StatusBadge status={node.status} />{node.last_error && <span className="cell-error">{node.last_error}</span>}</td>
                <td data-label="最后上报"><ResourceDate value={node.last_report_at} formatted={formatPreciseDate(node.last_report_at)} /></td>
                <td className="row-actions" data-label="操作"><div className="clear-row-actions">
                  <Link className="clear-row-edit" to={editorHref("/nodes", node.id)} aria-label={`编辑 ${node.name}`}>编辑</Link>
                  {(node.has_draft || node.status === "draft" || node.status === "error") && <button type="button" className="icon-button" aria-label={`发布 ${node.name}`} title="发布配置" disabled={publishing === node.id} onClick={() => void publish(node)}>{publishing === node.id ? <span className="mini-spinner" /> : <Rocket size={16} />}</button>}
                  <RowMenu className="resource-menu" label={`${node.name} 更多操作`}>
                    <Link to={editorHref("/nodes", node.id).replace("/edit?", "/history?")}>版本历史</Link>
                    <Link to={editorHref("/nodes", node.id)}><Edit3 size={15} />编辑节点</Link>
                    <Link to={editorHref("/nodes", "new", node.id)}><Copy size={15} />复制节点</Link>
                    <button disabled={changingStatus === node.id} onClick={() => setStatusTarget(node)}><Power size={15} />{node.status === "disabled" ? "恢复为草稿" : "停用节点"}</button>
                    <button type="button" className="menu-action--danger" onClick={() => setDeleteTarget(node)}><Trash2 size={15} />删除节点</button>
                  </RowMenu>
                </div></td>
              </tr>)}</tbody>
            </table></div>
          )}
        </div>
        {!loading && !error && <ResourceListFooter visible={filtered.length} total={nodes.length} />}
      </ResourceListSurface>
      <ResourceListNote>配置版本按“已应用 / 当前”显示。修改保存后需要发布，配置才会下发到服务器。</ResourceListNote>
      <ConfirmDialog open={Boolean(statusTarget)} title={statusTarget?.status === "disabled" ? "恢复为草稿？" : "停用这个节点？"} description="停用会影响订阅及运行配置；恢复后需要重新发布。" confirmLabel={statusTarget?.status === "disabled" ? "恢复为草稿" : "确认停用"} loading={Boolean(changingStatus)} onClose={() => setStatusTarget(null)} onConfirm={() => { if (statusTarget) void toggleStatus(statusTarget); }} />
      <ConfirmDialog open={Boolean(deleteTarget)} title="删除这个节点？" description={deleteTarget ? `“${deleteTarget.name}”将从节点列表、订阅和 Agent 配置中移除，指标与审计历史仍会保留。` : ""} confirmLabel="删除节点" loading={deleting} onClose={() => setDeleteTarget(null)} onConfirm={() => void deleteNode()} />
    </div>
  );
}
