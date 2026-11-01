import { useMemo, useState } from "react";
import {
  Copy,
  Edit3,
  Filter,
  Plus,
  Power,
  Rocket,
  Search,
  Trash2,
} from "lucide-react";
import { Link, useSearchParams } from "wouter";
import { ApiError, api, demoMode } from "../api";
import {
  Button,
  ConfirmDialog,
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
import type { Machine, Node } from "../types";

export function NodesPage() {
  const {
    data: nodes,
    loading,
    error,
    reload,
  } = useResource<Node[]>("/nodes", []);
  const { data: machines } = useResource<Machine[]>("/machines", []);
  const [params, setParams] = useSearchParams();
  const [search, setSearch] = useState(params.get("q") ?? "");
  const [machine, setMachine] = useState(params.get("machine") ?? "");
  const [status, setStatus] = useState(params.get("status") ?? "");
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
        (!status || node.status === status),
    );
  }, [nodes, search, machine, status]);

  function syncFilters(next: {
    search?: string;
    machine?: string;
    status?: string;
  }) {
    const values = { search, machine, status, ...next };
    const query: Record<string, string> = {};
    if (demoMode) query.demo = "1";
    if (values.search) query.q = values.search;
    if (values.machine) query.machine = values.machine;
    if (values.status) query.status = values.status;
    setParams(query, { replace: true });
  }

  async function publish(node: Node) {
    setPublishing(node.id);
    try {
      await api.post(`/nodes/${node.id}/publish`);
      toast(`${node.name} 已发布`);
      await reload();
    } catch (reason) {
      toast(
        reason instanceof ApiError ? reason.message : "发布失败，请重试",
        "error",
      );
    } finally {
      setPublishing("");
    }
  }

  async function toggleStatus(node: Node) {
    setChangingStatus(node.id);
    const nextStatus = node.status === "disabled" ? "draft" : "disabled";
    try {
      await api.patch(`/nodes/${node.id}`, { status: nextStatus });
      toast(nextStatus === "disabled" ? `${node.name} 已停用` : `${node.name} 已恢复为草稿`);
      await reload();
    } catch (reason) {
      toast(reason instanceof ApiError ? reason.message : "节点状态修改失败", "error");
    } finally {
      setChangingStatus("");
    }
  }

  async function deleteNode() {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await api.delete(`/nodes/${deleteTarget.id}`);
      toast(`${deleteTarget.name} 已删除`);
      setDeleteTarget(null);
      await reload();
    } catch (reason) {
      toast(reason instanceof ApiError ? reason.message : "节点删除失败，请重试", "error");
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div className="page clear-resources clear-resources--nodes">
      <PageHeader
        title="节点"
        description="维护 Corade 运行的入站服务配置"
        actions={<Link className="button button--primary" to="/nodes/new"><Plus size={16} />添加节点</Link>}
      />
      <ResourceListSurface>
        <ResourceCategoryFilters label="节点分类筛选" value={status} options={[
          { value: "", label: "全部", count: nodes.length },
          ...[{ value: "published", label: "已发布" }, { value: "draft", label: "待发布" }, { value: "disabled", label: "已停用" }, { value: "error", label: "异常" }].map((option) => ({ ...option, count: nodes.filter((node) => node.status === option.value).length })),
        ]} onChange={(value) => { setStatus(value); syncFilters({ status: value }); }} />
        <div className="toolbar">
          <label className="search-box"><Search size={16} aria-hidden="true" /><span className="sr-only">搜索节点</span><input value={search} onChange={(event) => { setSearch(event.target.value); syncFilters({ search: event.target.value }); }} placeholder="搜索名称、服务器、协议或端口" /></label>
          <div className="toolbar__filters"><Filter size={15} aria-hidden="true" /><select aria-label="按服务器筛选" value={machine} onChange={(event) => { setMachine(event.target.value); syncFilters({ machine: event.target.value }); }}><option value="">全部服务器</option>{machines.map((item) => <option value={item.id} key={item.id}>{item.name}</option>)}</select></div>
          <span className="toolbar__count">{filtered.length} 个节点</span>
        </div>
        <div className="table-surface">
          {loading ? <TableSkeleton columns={7} /> : error ? <EmptyState title="节点加载失败" description={error} action={<Button onClick={() => void reload()}>重新加载</Button>} /> : filtered.length === 0 ? <EmptyState title={nodes.length ? "没有符合筛选条件的节点" : "还没有节点"} description={nodes.length ? "调整搜索词或筛选条件后重试。" : "添加第一个节点并绑定到已经接入的服务器。"} action={nodes.length ? <Button onClick={() => { setSearch(""); setMachine(""); setStatus(""); syncFilters({ search: "", machine: "", status: "" }); }}>清除筛选</Button> : <Link className="button button--primary" to="/nodes/new"><Plus size={16} />添加节点</Link>} /> : (
            <div className="table-scroll resource-table-scroll"><table aria-label="节点列表">
              <colgroup><col style={{ width: "23%" }} /><col style={{ width: "14%" }} /><col style={{ width: "16%" }} /><col style={{ width: "12%" }} /><col style={{ width: "10%" }} /><col /><col style={{ width: "14%" }} /></colgroup>
              <thead><tr><th scope="col">节点</th><th scope="col">服务器</th><th scope="col">协议 / 监听</th><th scope="col">配置版本</th><th scope="col">状态</th><th scope="col">最后上报</th><th scope="col" className="col-actions">操作</th></tr></thead>
              <tbody>{filtered.map((node) => <tr key={node.id}>
                <td data-label="节点"><Link className="resource-name" to={`/nodes/${node.id}/edit`}>{node.name}</Link><span className="resource-id">#{node.agent_id} · {node.id}</span></td>
                <td data-label="服务器"><strong className="cell-primary">{node.machine_name}</strong></td>
                <td data-label="协议 / 监听"><span className="protocol-label">{node.protocol}</span><span className="resource-id mono">{node.listen_ip}:{node.server_port}</span></td>
                <td data-label="配置版本"><span className="mono">{node.applied_revision} / {node.current_revision}</span><span className="resource-id">{node.kernel_type === "singbox" ? "sing-box" : "Xray"}</span></td>
                <td data-label="状态"><StatusBadge status={node.status} />{node.last_error && <span className="cell-error">{node.last_error}</span>}</td>
                <td data-label="最后上报"><ResourceDate value={node.last_report_at} formatted={formatPreciseDate(node.last_report_at)} /></td>
                <td className="row-actions" data-label="操作"><div className="clear-row-actions">
                  <Link className="clear-row-edit" to={`/nodes/${node.id}/edit`} aria-label={`编辑 ${node.name}`}>编辑</Link>
                  {node.status !== "published" && <button type="button" className="icon-button" aria-label={`发布 ${node.name}`} title="发布配置" disabled={publishing === node.id} onClick={() => void publish(node)}>{publishing === node.id ? <span className="mini-spinner" /> : <Rocket size={16} />}</button>}
                  <RowMenu className="clear-resource-menu" label={`${node.name} 更多操作`}>
                    <Link to={`/nodes/${node.id}/edit`}><Edit3 size={15} />编辑节点</Link>
                    <Link to={`/nodes/new?copy=${node.id}`}><Copy size={15} />复制节点</Link>
                    <button disabled={changingStatus === node.id} onClick={() => void toggleStatus(node)}><Power size={15} />{node.status === "disabled" ? "恢复为草稿" : "停用节点"}</button>
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
      <ConfirmDialog open={Boolean(deleteTarget)} title="删除这个节点？" description={deleteTarget ? `“${deleteTarget.name}”将从节点列表、订阅和 Agent 配置中移除，指标与审计历史仍会保留。` : ""} confirmLabel="删除节点" loading={deleting} onClose={() => setDeleteTarget(null)} onConfirm={() => void deleteNode()} />
    </div>
  );
}
