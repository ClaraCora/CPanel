import { useEffect, useMemo, useRef, useState } from "react";
import { ArrowLeft, History, RefreshCw, RotateCcw } from "lucide-react";
import { Link, useLocation, useRoute } from "wouter";
import { api } from "../api";
import { Button, ConfirmDialog, EmptyState, PageHeader, TableSkeleton, formatPreciseDate, useToast } from "../components/ui";
import { useResource } from "../hooks";
import { editorHref, safeReturnTo } from "../navigation";
import { redactRevision, revisionDiff } from "../nodeRevisionDiff";
import { requestErrorMessage } from "../resourceModel";
import type { Node } from "../types";
import "../node-history.css";

type Revision = { revision: number; created_at: string; published_by: string | null; restorable: boolean; config: unknown; snapshot?: { node: Record<string, unknown>; spec: { settings: unknown } } };
function effectiveConfiguration(item: Revision | undefined) {
  if (!item) return {};
  return item.snapshot ? { ...item.snapshot.node, config: item.snapshot.spec.settings } : { config: item.config };
}

export function NodeHistoryPage() {
  const [, params] = useRoute<{ id: string }>("/nodes/:id/history");
  const id = params?.id ?? "";
  const [, navigate] = useLocation();
  const returnTo = safeReturnTo("/nodes");
  const history = useResource<Revision[]>(`/nodes/${encodeURIComponent(id)}/revisions`, []);
  const node = useResource<(Node & { has_draft?: boolean; config_version?: number }) | null>(`/nodes/${encodeURIComponent(id)}`, null);
  const [selected, setSelected] = useState<number | null>(null);
  const [baseline, setBaseline] = useState<number | null>(null);
  const [confirm, setConfirm] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [error, setError] = useState("");
  const lock = useRef(false);
  const toast = useToast();
  useEffect(() => {
    if (!history.data.length) return;
    setSelected((value) => history.data.some((item) => item.revision === value) ? value : history.data[0].revision);
    setBaseline((value) => history.data.some((item) => item.revision === value) ? value : (history.data[1]?.revision ?? history.data[0].revision));
  }, [history.data]);
  const current = history.data.find((item) => item.revision === selected);
  const previous = history.data.find((item) => item.revision === baseline);
  const changes = useMemo(() => revisionDiff(effectiveConfiguration(previous), effectiveConfiguration(current)), [current, previous]);

  async function restore() {
    if (!current?.restorable || lock.current) return;
    lock.current = true; setRestoring(true); setError("");
    try {
      await api.post(`/nodes/${encodeURIComponent(id)}/restore`, { revision: current.revision });
      setConfirm(false);
      toast(`版本 ${current.revision} 已恢复为草稿，尚未发布`);
      const target = new URL(editorHref("/nodes", id), location.origin);
      target.searchParams.set("returnTo", returnTo);
      navigate(target.pathname + target.search, { replace: true });
    } catch (reason) { setError(requestErrorMessage(reason, "恢复失败，请重试")); setConfirm(false); }
    finally { lock.current = false; setRestoring(false); }
  }

  return <div className="page resource-page node-history-page">
    <div className="back-row"><Link to={returnTo}><ArrowLeft size={16} />返回节点列表</Link></div>
    <PageHeader title="节点版本历史" description={`${node.data?.name ?? "节点"} · 比较发布内容，将历史版本恢复为草稿后再审核发布。`} actions={<Button onClick={() => { void history.reload(); void node.reload(); }} disabled={history.loading || node.loading}><RefreshCw size={16} />刷新</Button>} />
    {error && <div className="form-alert" role="alert">{error}</div>}
    {history.loading || node.loading ? <TableSkeleton columns={4} rows={5} /> : history.error || node.error ? <EmptyState title="版本历史加载失败" description={history.error || node.error} action={<Button onClick={() => { void history.reload(); void node.reload(); }}>重新加载</Button>} /> : !history.data.length ? <EmptyState title="还没有发布记录" description="首次发布后会保存完整配置快照。" action={<Link className="button button--secondary" to={editorHref("/nodes", id)}>编辑节点</Link>} /> : <div className="revision-layout">
      <section className="revision-history" aria-label="发布记录"><h2><History size={18} />发布记录</h2>{history.data.map((item) => <button type="button" key={item.revision} className={`revision-entry ${item.revision === selected ? "is-active" : ""}`} aria-pressed={item.revision === selected} onClick={() => { setSelected(item.revision); setError(""); }}><strong>版本 {item.revision}{item.revision === node.data?.current_revision && <span>当前发布</span>}</strong><time dateTime={item.created_at}>{formatPreciseDate(item.created_at)}</time><small>{item.restorable ? "完整配置快照" : "早期记录 · 仅可查看"}</small></button>)}</section>
      <section className="revision-content" aria-labelledby="revision-title"><div className="revision-heading"><h2 id="revision-title">版本 {selected}</h2><Button variant="primary" onClick={() => setConfirm(true)} disabled={!current?.restorable || restoring}><RotateCcw size={16} />恢复为草稿</Button></div>
        <p className="revision-explanation">完整快照包括协议、监听、入口及已解析的路由和出站。恢复时会将当时的路由保存在本节点配置内，并解除共享路由绑定；不会修改其他节点或立即改变当前服务。</p>
        {!current?.restorable && <p className="form-alert" role="status">此记录缺少完整依赖快照，无法保证还原当时运行状态，因此只提供查看。</p>}
        <label className="revision-compare">比较基准<select value={baseline ?? ""} onChange={(event) => setBaseline(Number(event.target.value))}>{history.data.map((item) => <option key={item.revision} value={item.revision}>版本 {item.revision}{item.restorable ? "" : "（部分记录）"}</option>)}</select></label>
        <p className="revision-count" role="status">版本 {baseline} → 版本 {selected}，{changes.length} 项差异。敏感值已隐藏。</p>
        {changes.length ? <div className="table-scroll"><table aria-label="配置差异"><thead><tr><th>字段</th><th>基准版本</th><th>所选版本</th></tr></thead><tbody>{changes.map((change) => <tr key={change.path}><th scope="row" data-label="字段"><code>{change.path}</code></th><td data-label="基准版本"><pre>{change.before}</pre></td><td data-label="所选版本"><pre>{change.after}</pre></td></tr>)}</tbody></table></div> : <p>两份记录没有可比较的配置差异。</p>}
        <details className="revision-json"><summary>查看所选版本完整结构（敏感值已隐藏）</summary><pre>{JSON.stringify(redactRevision(effectiveConfiguration(current)), null, 2)}</pre></details>
      </section>
    </div>}
    <ConfirmDialog open={confirm} title={`恢复版本 ${selected} 为草稿？`} description={`${node.data?.has_draft ? "当前未发布草稿将被替换。" : ""}恢复后进入完整编辑页，确认并发布后才会应用。路由将使用该版本独立快照，不再跟随原共享策略变化。`} confirmLabel="恢复并编辑" confirmVariant="primary" loading={restoring} onClose={() => { if (!restoring) setConfirm(false); }} onConfirm={() => void restore()} />
  </div>;
}
