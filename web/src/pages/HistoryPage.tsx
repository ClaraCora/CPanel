import { useState } from "react";
import { Clock3, Settings } from "lucide-react";
import { Link } from "wouter";
import { Button, EmptyState, PageHeader, Pagination, StatusBadge, TableSkeleton, formatBytes, formatDate } from "../components/ui";
import { usePagedResource, useResource } from "../hooks";
import type { DailyTraffic, DeviceHistory, HistoricalData, MetricSample } from "../types";

const emptyHistory: HistoricalData = {
  retention: { devices_days: 30, traffic_days: 90 },
  traffic: [],
  machine_metrics: [],
  node_metrics: [],
  devices: [],
};

type View = "traffic" | "machines" | "nodes" | "devices";

export function HistoryPage() {
  const { data, loading, error, reload } = useResource<HistoricalData>("/history", emptyHistory);
  const [view, setView] = useState<View>("traffic");
  const paged = usePagedResource<DailyTraffic | MetricSample | DeviceHistory>(`/history?view=${view}`);
  const retentionLabel = view === "traffic" ? `保留 ${data.retention.traffic_days} 天` : view === "devices" ? `保留 ${data.retention.devices_days} 天` : "仅保留最新值";

  return <div className="page">
    <PageHeader title="历史数据" description="查看保留期内的流量、运行指标与设备连接记录" actions={<Link className="button button--secondary" to="/settings"><Settings size={16} />保留设置</Link>} />
    <div className="history-toolbar">
      <div className="history-tabs" role="tablist" aria-label="历史数据类型">
        <Tab active={view === "traffic"} onClick={() => setView("traffic")}>每日流量</Tab>
        <Tab active={view === "machines"} onClick={() => setView("machines")}>服务器指标</Tab>
        <Tab active={view === "nodes"} onClick={() => setView("nodes")}>节点指标</Tab>
        <Tab active={view === "devices"} onClick={() => setView("devices")}>设备记录</Tab>
      </div>
      <span className="retention-label"><Clock3 size={15} />{retentionLabel}</span>
    </div>
    {loading || paged.loading ? <TableSkeleton columns={5} rows={8} /> : error || paged.error ? <EmptyState title="历史数据加载失败" description={error || paged.error} action={<Button onClick={() => { void reload(); void paged.reload(); }}>重新加载</Button>} /> : view === "traffic" ? <TrafficTable items={paged.data as DailyTraffic[]} page={paged.page} pageSize={paged.pageSize} total={paged.total} hasMore={paged.hasMore} onPageChange={paged.setPage} /> : view === "machines" ? <MetricsTable title="服务器指标" items={paged.data as MetricSample[]} page={paged.page} pageSize={paged.pageSize} total={paged.total} hasMore={paged.hasMore} onPageChange={paged.setPage} /> : view === "nodes" ? <MetricsTable title="节点指标" items={paged.data as MetricSample[]} page={paged.page} pageSize={paged.pageSize} total={paged.total} hasMore={paged.hasMore} onPageChange={paged.setPage} /> : <DeviceTable items={paged.data as DeviceHistory[]} page={paged.page} pageSize={paged.pageSize} total={paged.total} hasMore={paged.hasMore} onPageChange={paged.setPage} />}
  </div>;
}

function Tab({ active, onClick, children }: { active: boolean; onClick: () => void; children: string }) {
  return <button type="button" role="tab" aria-selected={active} className={active ? "active" : ""} onClick={onClick}>{children}</button>;
}

function TrafficTable({ items, page, pageSize, total, hasMore, onPageChange }: { items: DailyTraffic[]; page: number; pageSize: number; total: number; hasMore: boolean; onPageChange: (page: number) => void }) {
  if (!items.length) return <HistoryEmpty title="保留期内没有流量记录" />;
  return <div className="table-surface table-surface--standalone"><div className="table-scroll"><table><thead><tr><th>日期</th><th>上传</th><th>下载</th><th>合计</th></tr></thead><tbody>{items.map((item) => <tr key={item.day}><td><strong className="cell-primary">{new Date(item.day).toLocaleDateString("zh-CN")}</strong></td><td className="mono">{formatBytes(item.upload_bytes)}</td><td className="mono">{formatBytes(item.download_bytes)}</td><td className="mono"><strong>{formatBytes(item.upload_bytes + item.download_bytes)}</strong></td></tr>)}</tbody></table></div><Pagination page={page} pageSize={pageSize} total={total} hasMore={hasMore} onPageChange={onPageChange} /></div>;
}

function MetricsTable({ title, items, page, pageSize, total, hasMore, onPageChange }: { title: string; items: MetricSample[]; page: number; pageSize: number; total: number; hasMore: boolean; onPageChange: (page: number) => void }) {
  if (!items.length) return <HistoryEmpty title={`保留期内没有${title}记录`} />;
  return <div className="table-surface table-surface--standalone"><div className="table-scroll"><table><thead><tr><th>{title === "服务器指标" ? "服务器" : "节点"}</th><th>采样时间</th><th>指标摘要</th></tr></thead><tbody>{items.map((item, index) => <tr key={`${item.resource_id}-${item.sampled_at}-${index}`}><td><strong className="cell-primary">{item.resource_name}</strong><span className="resource-id">{item.resource_id}</span></td><td>{formatDate(item.sampled_at)}</td><td><MetricSummary metrics={item.metrics} /></td></tr>)}</tbody></table></div><Pagination page={page} pageSize={pageSize} total={total} hasMore={hasMore} onPageChange={onPageChange} /></div>;
}

function MetricSummary({ metrics }: { metrics: Record<string, unknown> }) {
  const entries = Object.entries(metrics ?? {}).filter(([, value]) => ["string", "number", "boolean"].includes(typeof value)).slice(0, 6);
  if (!entries.length) return <span className="history-muted">无可显示指标</span>;
  return <div className="metric-summary">{entries.map(([key, value]) => <span key={key}><b>{metricLabel(key)}</b>{formatMetricValue(key, value)}</span>)}</div>;
}

function DeviceTable({ items, page, pageSize, total, hasMore, onPageChange }: { items: DeviceHistory[]; page: number; pageSize: number; total: number; hasMore: boolean; onPageChange: (page: number) => void }) {
  if (!items.length) return <HistoryEmpty title="保留期内没有设备连接记录" />;
  return <div className="table-surface table-surface--standalone"><div className="table-scroll"><table><thead><tr><th>账号</th><th>节点</th><th>IP 地址</th><th>首次出现</th><th>最后出现</th><th>状态</th></tr></thead><tbody>{items.map((item) => <tr key={item.id}><td><strong className="cell-primary">{item.user_name}</strong><span className="resource-id">{item.user_id}</span></td><td><strong className="cell-primary">{item.node_name}</strong><span className="resource-id">{item.node_id}</span></td><td className="mono">{item.ip_address}</td><td>{formatDate(item.first_seen_at)}</td><td>{formatDate(item.last_seen_at)}</td><td><StatusBadge status={item.online ? "online" : "offline"} /></td></tr>)}</tbody></table></div><Pagination page={page} pageSize={pageSize} total={total} hasMore={hasMore} onPageChange={onPageChange} /></div>;
}

function HistoryEmpty({ title }: { title: string }) {
  return <div className="table-surface table-surface--standalone"><EmptyState title={title} description="Agent 上报数据后会显示在这里。指标只保留每台服务器或节点的最新值。" /></div>;
}

function metricLabel(key: string) {
  const labels: Record<string, string> = { cpu: "CPU", cpu_percent: "CPU", memory: "内存", memory_percent: "内存", memory_used_bytes: "已用内存", load: "负载", load_1: "1 分钟负载", connections: "连接数", upload_bytes: "上传", download_bytes: "下载", uptime_seconds: "运行时间" };
  return labels[key] ?? key.replaceAll("_", " ");
}

function formatMetricValue(key: string, value: unknown) {
  if (typeof value === "number" && key.endsWith("_bytes")) return formatBytes(value);
  if (typeof value === "number" && key.includes("percent")) return `${value.toLocaleString("zh-CN", { maximumFractionDigits: 1 })}%`;
  if (typeof value === "number") return value.toLocaleString("zh-CN", { maximumFractionDigits: 2 });
  return String(value);
}
