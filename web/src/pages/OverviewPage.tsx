import {
  AlertTriangle,
  ArrowRight,
  ArrowDownToLine,
  ArrowUpFromLine,
  Boxes,
  LoaderCircle,
  Server,
  Users,
} from "lucide-react";
import { useState } from "react";
import { Link } from "wouter";
import { useResource } from "../hooks";
import type { Overview, TrafficRank } from "../types";
import {
  EmptyState,
  PageHeader,
  TableSkeleton,
  formatBytes,
  DataFreshness,
} from "../components/ui";

const emptyOverview: Overview = {
  machines_total: 0,
  machines_online: 0,
  machines_offline: 0,
  nodes_total: 0,
  nodes_published: 0,
  admins_active: 0,
  users_active: 0,
  friends_active: 0,
  traffic_today_bytes: 0,
  traffic_today_upload_bytes: 0,
  traffic_today_download_bytes: 0,
  node_traffic_ranking: [],
  user_traffic_ranking: [],
};

const rankingPeriods = [
  { value: "today", label: "今日" },
  { value: "yesterday", label: "昨日" },
  { value: "7d", label: "近七天" },
  { value: "month", label: "本月" },
] as const;

type RankingPeriod = (typeof rankingPeriods)[number]["value"];

export function OverviewPage() {
  const [period, setPeriod] = useState<RankingPeriod>("today");
  const { data, loading, error, reload, lastSuccessAt, refreshing, stale, refreshError } = useResource<Overview>(
    `/overview?period=${period}`,
    emptyOverview,
    60_000,
  );
  const hasLoaded = data !== emptyOverview;
  const periodLabel =
    rankingPeriods.find((item) => item.value === period)?.label ?? "今日";
  return (
    <div className="page">
      <PageHeader
        title="运行总览"
        description="服务器、节点与订阅账号的当前状态"
        actions={
          <Link className="button button--primary" to="/machines">
            管理服务器 <ArrowRight size={16} />
          </Link>
        }
      />
      {error && !hasLoaded ? (
        <EmptyState
          title="总览加载失败"
          description={error}
          action={
            <button
              className="button button--secondary"
              onClick={() => void reload()}
            >
              重新加载
            </button>
          }
        />
      ) : loading && !hasLoaded ? (
        <TableSkeleton columns={4} rows={2} />
      ) : (
        <>
          <section className="metric-band" aria-label="运行指标">
            <Metric
              icon={ArrowUpFromLine}
              label="今日上传"
              value={formatBytes(data.traffic_today_upload_bytes)}
              detail="Agent 已确认流量"
            />
            <Metric
              icon={ArrowDownToLine}
              label="今日下载"
              value={formatBytes(data.traffic_today_download_bytes)}
              detail={`合计 ${formatBytes(data.traffic_today_bytes)}`}
            />
            <Metric
              icon={Server}
              label="服务器"
              value={data.machines_total}
              detail={`${data.machines_online} 在线 / ${data.machines_offline} 离线`}
              tone={data.machines_offline ? "danger" : "success"}
            />
            <Metric
              icon={Boxes}
              label="节点"
              value={data.nodes_total}
              detail={`${data.nodes_published} 已发布`}
            />
            <Metric
              icon={Users}
              label="订阅账号"
              value={data.admins_active + data.users_active + data.friends_active}
              detail={`${data.admins_active} 管理员 / ${data.users_active} 用户 / ${data.friends_active} 朋友`}
            />
          </section>
          <section className="ranking-section" aria-labelledby="traffic-ranking-title">
            <div className="section-heading">
              <div>
                <h2 id="traffic-ranking-title">{periodLabel}流量排行</h2>
                <p>按上传与下载合计排序，最多显示 10 条。</p>
              </div>
              <div className="overview-heading-actions">
              <DataFreshness lastSuccessAt={lastSuccessAt} stale={stale} refreshing={refreshing} error={refreshError} />
              <div
                className="ranking-period"
                role="group"
                aria-label="流量排行时间范围"
              >
                {rankingPeriods.map((item) => (
                  <button
                    type="button"
                    className={period === item.value ? "active" : ""}
                    aria-pressed={period === item.value}
                    onClick={() => setPeriod(item.value)}
                    key={item.value}
                  >
                    {item.label}
                  </button>
                ))}
                {loading && (
                  <LoaderCircle
                    className="spin ranking-period__loading"
                    size={15}
                    aria-label="正在更新排行"
                  />
                )}
              </div></div>
            </div>
            {error && (
              <div className="ranking-error" role="alert">
                <span>{error}</span>
                <button type="button" onClick={() => void reload()}>
                  重新加载
                </button>
              </div>
            )}
            <div className={`ranking-grid ${loading ? "is-loading" : ""}`}>
              <TrafficRanking title="节点流量排行" items={data.node_traffic_ranking} empty={`${periodLabel}还没有节点流量`} />
              <TrafficRanking title="用户流量排行" items={data.user_traffic_ranking} empty={`${periodLabel}还没有用户流量`} />
            </div>
          </section>
          <section className="overview-section">
            <div className="section-heading">
              <div>
                <h2>需要关注</h2>
                <p>优先处理离线服务器与未发布节点。</p>
              </div>
            </div>
            {data.machines_offline === 0 &&
            data.nodes_total === data.nodes_published ? (
              <EmptyState
                title="当前没有待处理事项"
                description="服务器在线且所有节点配置均已发布。"
              />
            ) : (
              <div className="attention-list">
                {data.machines_offline > 0 && (
                  <Link to="/machines" className="attention-row">
                    <AlertTriangle size={18} className="text-danger" />
                    <div>
                      <strong>{data.machines_offline} 台服务器离线</strong>
                      <span>检查最后心跳、Agent 进程与接入凭据。</span>
                    </div>
                    <ArrowRight size={17} />
                  </Link>
                )}
                {data.nodes_total > data.nodes_published && (
                  <Link to="/nodes?status=draft" className="attention-row">
                    <Boxes size={18} className="text-warning" />
                    <div>
                      <strong>
                        {data.nodes_total - data.nodes_published} 个节点等待发布
                      </strong>
                      <span>检查草稿配置并发布到对应服务器。</span>
                    </div>
                    <ArrowRight size={17} />
                  </Link>
                )}
              </div>
            )}
          </section>
        </>
      )}
    </div>
  );
}

function TrafficRanking({ title, items, empty }: { title: string; items: TrafficRank[]; empty: string }) {
  return <div className="traffic-ranking"><h3>{title}</h3>{items.length === 0 ? <p className="ranking-empty">{empty}</p> : <div className="table-scroll"><table><thead><tr><th>#</th><th>名称</th><th>上传</th><th>下载</th><th>合计</th></tr></thead><tbody>{items.map((item, index) => <tr key={item.id}><td className="mono">{index + 1}</td><td><strong className="cell-primary">{item.name}</strong></td><td className="mono">{formatBytes(item.upload_bytes)}</td><td className="mono">{formatBytes(item.download_bytes)}</td><td className="mono"><strong>{formatBytes(item.total_bytes)}</strong></td></tr>)}</tbody></table></div>}</div>;
}

function Metric({
  icon: Icon,
  label,
  value,
  detail,
  tone,
}: {
  icon: typeof Server;
  label: string;
  value: string | number;
  detail: string;
  tone?: string;
}) {
  return (
    <div className="metric-item">
      <Icon size={18} aria-hidden="true" />
      <div>
        <span>{label}</span>
        <strong>{value}</strong>
        <small className={tone ? `text-${tone}` : ""}>{detail}</small>
      </div>
    </div>
  );
}
