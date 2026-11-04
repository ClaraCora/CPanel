import { useState } from "react";
import { Copy, RefreshCw, Search } from "lucide-react";
import { Button, DetailDialog, EmptyState, PageHeader, Pagination, TableSkeleton, formatDateWithYear, useToast } from "../components/ui";
import { ResourceDate, ResourceListNote } from "../components/ResourceList";
import { usePagedResource } from "../hooks";
import { auditActionLabels as actions } from "../auditActions";

type AuditEvent = { id: string; admin_id: string | null; admin_name: string | null; action: string; resource_type: string; resource_id: string; changes: Record<string, unknown>; ip_address: string | null; request_id: string; created_at: string };

export function AuditPage() {
  const { data, loading, error, reload, query, setQuery, page, pageSize, total, hasMore, setPage } = usePagedResource<AuditEvent>("/audit-events");
  const [selected, setSelected] = useState<AuditEvent | null>(null);
  const toast = useToast();
  async function copyRequest(value: string) {
    try { await navigator.clipboard.writeText(value); toast("请求 ID 已复制"); }
    catch { toast("复制失败，请手动选择请求 ID", "error"); }
  }
  return <div className="page resource-page operations-page admin-audit">
    <PageHeader title="审计日志" description="追溯管理员登录、配置变更、发布与凭据操作" actions={<Button disabled={loading} onClick={() => void reload()}><RefreshCw size={15} />刷新</Button>} />
    <section className="clear-list">
      <div className="toolbar"><label className="search-box"><Search size={16} /><span className="sr-only">搜索审计日志</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索操作、资源或请求 ID" /></label><select aria-label="按审计操作筛选" value={Object.values(actions).includes(query) ? query : ""} onChange={(event) => setQuery(event.target.value)}><option value="">全部操作</option>{Object.entries(actions).map(([action, label]) => <option key={action} value={label}>{label}</option>)}</select><span className="toolbar__count">共 {total} 条记录</span></div>
      <div className="table-surface">{loading ? <TableSkeleton columns={7} /> : error ? <EmptyState title="审计日志加载失败" description={error} action={<Button onClick={() => void reload()}>重新加载</Button>} /> : data.length === 0 ? <EmptyState title={query ? "没有匹配的审计日志" : "暂无审计日志"} description={query ? "请调整搜索内容。" : "管理员执行操作后，记录会显示在这里。"} /> : <>
        <div className="table-scroll"><table><colgroup><col style={{ width: "15%" }} /><col style={{ width: "12%" }} /><col style={{ width: "19%" }} /><col style={{ width: "17%" }} /><col style={{ width: "15%" }} /><col /><col style={{ width: 68 }} /></colgroup><thead><tr><th>时间</th><th>管理员</th><th>操作</th><th>资源</th><th>来源 IP</th><th>请求 ID</th><th>操作</th></tr></thead><tbody>{data.map((item) => <tr key={item.id}>
          <td data-label="时间"><ResourceDate value={item.created_at} formatted={formatDateWithYear(item.created_at)} /></td>
          <td data-label="管理员">{item.admin_name || "系统"}</td>
          <td data-label="操作"><strong className="cell-primary">{actions[item.action] || item.action}</strong><span className="resource-id mono">{item.action}</span></td>
          <td data-label="资源"><span>{item.resource_type}</span><span className="resource-id mono">{item.resource_id || "—"}</span></td>
          <td data-label="来源 IP" className="mono">{item.ip_address || "—"}</td>
          <td data-label="请求 ID" className="mono">{item.request_id || "—"}</td>
          <td data-label="详情" className="row-actions"><button type="button" className="clear-row-edit" aria-label={`查看 ${actions[item.action] || item.action} 的详情 ${item.id}`} onClick={() => setSelected(item)}>详情</button></td>
        </tr>)}</tbody></table></div>
      </>}</div>
      {!loading && !error && <Pagination page={page} pageSize={pageSize} total={total} hasMore={hasMore} onPageChange={setPage} />}
    </section>
    <ResourceListNote>可使用请求 ID 关联服务日志。密码、令牌与密钥等敏感内容已隐藏。</ResourceListNote>
    <DetailDialog className="resource-detail clear-operation-editor" open={Boolean(selected)} title="审计详情" description={selected ? actions[selected.action] || selected.action : undefined} onClose={() => setSelected(null)}>{selected && <div className="audit-detail"><dl className="operation-detail-list">
      <div><dt>发生时间</dt><dd>{formatDateWithYear(selected.created_at)}</dd></div><div><dt>管理员</dt><dd>{selected.admin_name || "系统"}<small>{selected.admin_id}</small></dd></div><div><dt>操作</dt><dd className="mono">{selected.action}</dd></div><div><dt>资源</dt><dd>{selected.resource_type}<small>{selected.resource_id || "—"}</small></dd></div><div><dt>来源 IP</dt><dd className="mono">{selected.ip_address || "—"}</dd></div><div><dt>请求 ID</dt><dd className="audit-request"><code>{selected.request_id || "—"}</code>{selected.request_id && <Button type="button" onClick={() => void copyRequest(selected.request_id)}><Copy size={14} />复制</Button>}</dd></div>
    </dl><h3>变更内容</h3><p className="operation-hint">显示服务端记录的操作参数；敏感字段已脱敏。</p><pre className="audit-json">{JSON.stringify(selected.changes ?? {}, (key, value: unknown) => /password|token|secret|private.?key|communication.?key|credential|^config$|^settings$/i.test(key) ? "[REDACTED]" : value, 2)}</pre></div>}</DetailDialog>
  </div>;
}
