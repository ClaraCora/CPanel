import { Search } from "lucide-react";
import { useMemo, useState } from "react";
import { Button, EmptyState, PageHeader, TableSkeleton, formatDate } from "../components/ui";
import { useResource } from "../hooks";

type AuditEvent = { id: string; admin_id: string | null; admin_name: string | null; action: string; resource_type: string; resource_id: string; changes: Record<string, unknown>; ip_address: string | null; request_id: string; created_at: string };
const actions: Record<string, string> = { "admin.login": "管理员登录", "admin.logout": "管理员退出", "admin.profile.update": "修改管理员资料", "admin.password.update": "修改管理员密码", "node.create": "创建节点", "node.update": "编辑节点", "node.publish": "发布节点", "machine.create": "添加服务器", "settings.update": "修改设置", "user.create": "添加账号" };

export function AuditPage() {
  const { data, loading, error, reload } = useResource<AuditEvent[]>("/audit-events", []);
  const [query, setQuery] = useState("");
  const filtered = useMemo(() => data.filter((item) => !query || JSON.stringify(item).toLowerCase().includes(query.toLowerCase())), [data, query]);
  return <div className="page"><PageHeader title="审计日志" description="管理员登录、配置变更、发布和凭据操作记录" /><div className="toolbar"><label className="search-box"><Search size={16} /><span className="sr-only">搜索审计日志</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索操作、资源或请求 ID" /></label><span className="toolbar__count">最近 {filtered.length} 条</span></div><div className="table-surface">{loading ? <TableSkeleton columns={6} /> : error ? <EmptyState title="审计日志加载失败" description={error} action={<Button onClick={() => void reload()}>重新加载</Button>} /> : filtered.length === 0 ? <EmptyState title="暂无审计日志" description="管理员执行敏感操作后，记录会显示在这里。" /> : <div className="table-scroll"><table><thead><tr><th>时间</th><th>管理员</th><th>操作</th><th>资源</th><th>来源 IP</th><th>请求 ID</th></tr></thead><tbody>{filtered.map((item) => <tr key={item.id}><td>{formatDate(item.created_at)}</td><td>{item.admin_name || "系统"}</td><td><strong className="cell-primary">{actions[item.action] || item.action}</strong></td><td><span>{item.resource_type}</span><span className="resource-id">{item.resource_id || "-"}</span></td><td className="mono">{item.ip_address || "-"}</td><td className="mono">{item.request_id || "-"}</td></tr>)}</tbody></table></div>}</div></div>;
}
