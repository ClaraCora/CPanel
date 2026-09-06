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
    <div className="page">
      <PageHeader
        title="节点"
        description="维护 Corade 运行的入站服务配置"
        actions={
          <Link className="button button--primary" to="/nodes/new">
            <Plus size={16} />
            添加节点
          </Link>
        }
      />
      <div className="toolbar">
        <label className="search-box">
          <Search size={16} aria-hidden="true" />
          <span className="sr-only">搜索节点</span>
          <input
            value={search}
            onChange={(event) => {
              setSearch(event.target.value);
              syncFilters({ search: event.target.value });
            }}
            placeholder="搜索名称、服务器、协议或端口"
          />
        </label>
        <div className="toolbar__filters">
          <Filter size={15} aria-hidden="true" />
          <select
            aria-label="按服务器筛选"
            value={machine}
            onChange={(event) => {
              setMachine(event.target.value);
              syncFilters({ machine: event.target.value });
            }}
          >
            <option value="">全部服务器</option>
            {machines.map((item) => (
              <option value={item.id} key={item.id}>
                {item.name}
              </option>
            ))}
          </select>
          <select
            aria-label="按状态筛选"
            value={status}
            onChange={(event) => {
              setStatus(event.target.value);
              syncFilters({ status: event.target.value });
            }}
          >
            <option value="">全部状态</option>
            <option value="published">已发布</option>
            <option value="draft">待发布</option>
            <option value="disabled">已停用</option>
            <option value="error">异常</option>
          </select>
        </div>
        <span className="toolbar__count">{filtered.length} 个节点</span>
      </div>
      <div className="table-surface">
        {loading ? (
          <TableSkeleton columns={8} />
        ) : error ? (
          <EmptyState
            title="节点加载失败"
            description={error}
            action={<Button onClick={() => void reload()}>重新加载</Button>}
          />
        ) : filtered.length === 0 ? (
          <EmptyState
            title={nodes.length ? "没有符合筛选条件的节点" : "还没有节点"}
            description={
              nodes.length
                ? "调整搜索词或筛选条件后重试。"
                : "添加第一个节点并绑定到已经接入的服务器。"
            }
            action={
              !nodes.length && (
                <Link className="button button--primary" to="/nodes/new">
                  <Plus size={16} />
                  添加节点
                </Link>
              )
            }
          />
        ) : (
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th className="col-check">
                    <input type="checkbox" aria-label="选择全部节点" />
                  </th>
                  <th>节点</th>
                  <th>服务器</th>
                  <th>协议</th>
                  <th>监听</th>
                  <th>内核</th>
                  <th>版本</th>
                  <th>状态</th>
                  <th>最后上报</th>
                  <th className="col-actions">操作</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((node) => (
                  <tr key={node.id}>
                    <td className="col-check">
                      <input type="checkbox" aria-label={`选择 ${node.name}`} />
                    </td>
                    <td>
                      <Link
                        className="resource-name"
                        to={`/nodes/${node.id}/edit`}
                      >
                        {node.name}
                      </Link>
                      <span className="resource-id">
                        #{node.agent_id} · {node.id}
                      </span>
                    </td>
                    <td>
                      <strong className="cell-primary">
                        {node.machine_name}
                      </strong>
                    </td>
                    <td>
                      <span className="protocol-label">{node.protocol}</span>
                    </td>
                    <td className="mono">
                      {node.listen_ip}:{node.server_port}
                    </td>
                    <td>
                      {node.kernel_type === "singbox" ? "sing-box" : "Xray"}
                    </td>
                    <td className="mono">
                      {node.applied_revision}/{node.current_revision}
                    </td>
                    <td>
                      <StatusBadge status={node.status} />
                      {node.last_error && (
                        <span className="cell-error">{node.last_error}</span>
                      )}
                    </td>
                    <td>{formatPreciseDate(node.last_report_at)}</td>
                    <td className="row-actions">
                      <Link
                        className="icon-button"
                        to={`/nodes/${node.id}/edit`}
                        aria-label={`编辑 ${node.name}`}
                        title="编辑"
                      >
                        <Edit3 size={16} />
                      </Link>
                      <Link
                        className="icon-button"
                        to={`/nodes/new?copy=${node.id}`}
                        aria-label={`复制 ${node.name}`}
                        title="复制"
                      >
                        <Copy size={16} />
                      </Link>
                      {node.status !== "published" && (
                        <button
                          className="icon-button"
                          aria-label={`发布 ${node.name}`}
                          title="发布"
                          disabled={publishing === node.id}
                          onClick={() => void publish(node)}
                        >
                          {publishing === node.id ? (
                            <span className="mini-spinner" />
                          ) : (
                            <Rocket size={16} />
                          )}
                        </button>
                      )}
                      <RowMenu label={`${node.name} 更多操作`}>
                        <Link to={`/nodes/${node.id}/edit`}><Edit3 size={15} />编辑节点</Link>
                        <Link to={`/nodes/new?copy=${node.id}`}><Copy size={15} />复制节点</Link>
                        <button disabled={changingStatus === node.id} onClick={() => void toggleStatus(node)}><Power size={15} />{node.status === "disabled" ? "恢复为草稿" : "停用节点"}</button>
                        <button type="button" className="menu-action--danger" onClick={() => setDeleteTarget(node)}><Trash2 size={15} />删除节点</button>
                      </RowMenu>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
      {!loading && !error && filtered.length > 0 && (
        <footer className="pagination">
          <span>
            显示 1-{filtered.length}，共 {filtered.length} 条
          </span>
          <div>
            <button disabled>上一页</button>
            <button className="pagination__current">1</button>
            <button disabled>下一页</button>
          </div>
        </footer>
      )}
      <ConfirmDialog
        open={Boolean(deleteTarget)}
        title="删除这个节点？"
        description={deleteTarget ? `“${deleteTarget.name}”将从节点列表、订阅和 Agent 配置中移除，指标与审计历史仍会保留。` : ""}
        confirmLabel="删除节点"
        loading={deleting}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => void deleteNode()}
      />
    </div>
  );
}
