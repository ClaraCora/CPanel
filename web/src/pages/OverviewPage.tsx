import { OverviewTraffic } from "../components/OverviewTraffic";
import {
  AlertTriangle,
  CheckCircle2,
  ArrowRight,
  Boxes,
  LoaderCircle,
  UsersRound,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useId } from "react";
import { Link, useSearchParams } from "wouter";
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
  const [params, setParams] = useSearchParams();
  const period = rankingPeriods.find((item) => item.value === params.get("period")) ?? rankingPeriods[0];
  function selectPeriod(value: RankingPeriod) {
    setParams((previous) => {
      const next = new URLSearchParams(previous);
      if (value === "today") next.delete("period");
      else next.set("period", value);
      return next;
    }, { replace: true });
  }
  const { data, loading, error, reload, lastSuccessAt, lastSuccessPath, refreshing, stale, refreshError } = useResource<Overview>(
    `/overview?period=${period.value}`,
    emptyOverview,
    60_000,
  );
  const hasLoaded = lastSuccessPath !== null;
  const shownPeriod = rankingPeriods.find((item) => lastSuccessPath === `/overview?period=${item.value}`) ?? period;
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
          <OverviewAttention data={data} />
          <OverviewTraffic data={data} />
          <section className="ranking-section" aria-labelledby="traffic-ranking-title">
            <div className="section-heading ranking-section__heading">
              <div>
                <h2 id="traffic-ranking-title">{shownPeriod.label}流量排行</h2>
                <p>按上传与下载合计排序，最多显示 10 条。条形长度以各榜榜首为基准。</p>
              </div>
              <div className="overview-heading-actions">
                {loading ? (
                  <span className="data-freshness ranking-section__loading" role="status">
                    <LoaderCircle className="spin" size={14} aria-hidden="true" />
                    正在加载{period.label}排行…
                  </span>
                ) : (
                  <DataFreshness lastSuccessAt={lastSuccessAt} stale={stale} refreshing={refreshing} error={refreshError} />
                )}
                <div className="ranking-period" role="group" aria-label="流量排行时间范围">
                  {rankingPeriods.map((item) => (
                    <button
                      type="button"
                      aria-pressed={period.value === item.value}
                      onClick={() => selectPeriod(item.value)}
                      key={item.value}
                    >
                      {item.label}
                    </button>
                  ))}
                </div>
              </div>
            </div>
            {error && (
              <div className="ranking-error">
                <p role="alert">{period.label}排行加载失败，当前显示{shownPeriod.label}排行。{error}</p>
                <button type="button" className="button button--secondary" onClick={() => void reload()}>
                  重新加载
                </button>
              </div>
            )}
            <div className="ranking-grid" aria-busy={loading}>
              <TrafficRanking title="节点流量排行" icon={Boxes} items={data.node_traffic_ranking} empty={`${shownPeriod.label}还没有节点流量`} />
              <TrafficRanking title="用户流量排行" icon={UsersRound} items={data.user_traffic_ranking} empty={`${shownPeriod.label}还没有用户流量`} />
            </div>
          </section>
        </>
      )}
    </div>
  );
}

function OverviewAttention({ data }: { data: Overview }) {
  const unpublished = data.nodes_total - data.nodes_published;
  if (!data.machines_offline && !unpublished) {
    const hasResources = data.machines_total > 0 || data.nodes_total > 0;
    return <p className="overview-health">
      {hasResources && <CheckCircle2 size={16} aria-hidden="true" />}
      {hasResources ? "当前没有离线服务器或未发布节点" : "尚未接入服务器或节点"}
    </p>;
  }
  return <section className="overview-attention" aria-labelledby="overview-attention-title">
    <h2 id="overview-attention-title">需要关注</h2>
    <div className="attention-list">
      {data.machines_offline > 0 && <Link to="/machines?status=offline" className="attention-row">
        <AlertTriangle size={18} className="text-danger" aria-hidden="true" />
        <div><strong>{data.machines_offline} 台服务器离线</strong><span>查看离线服务器和最后心跳</span></div>
        <ArrowRight size={16} aria-hidden="true" />
      </Link>}
      {unpublished > 0 && <Link to="/nodes" className="attention-row">
        <Boxes size={18} className="text-warning" aria-hidden="true" />
        <div><strong>{unpublished} 个节点未发布</strong><span>查看节点状态与配置</span></div>
        <ArrowRight size={16} aria-hidden="true" />
      </Link>}
    </div>
  </section>;
}

function TrafficRanking({ title, icon: Icon, items, empty }: {
  title: string;
  icon: LucideIcon;
  items: TrafficRank[];
  empty: string;
}) {
  const titleId = useId();
  const peak = Math.max(0, ...items.map((item) => item.total_bytes));

  return (
    <div className="traffic-ranking">
      <h3 id={titleId}><Icon size={18} aria-hidden="true" />{title}</h3>
      {items.length === 0 ? (
        <p className="ranking-empty">{empty}</p>
      ) : (
        <table role="table" aria-labelledby={titleId}>
          <thead role="rowgroup">
            <tr role="row">
              <th scope="col" role="columnheader">排名与名称</th>
              <th scope="col" role="columnheader" className="traffic-ranking__amount">上传</th>
              <th scope="col" role="columnheader" className="traffic-ranking__amount">下载</th>
              <th scope="col" role="columnheader" className="traffic-ranking__amount">合计</th>
            </tr>
          </thead>
          <tbody role="rowgroup">
            {items.map((item, index) => (
              <tr role="row" key={item.id}>
                <th scope="row" role="rowheader" className="traffic-ranking__identity">
                  <span className="traffic-ranking__rank">{index + 1}</span>
                  <span className="traffic-ranking__name">{item.name}</span>
                </th>
                <td role="cell" className="traffic-ranking__amount traffic-ranking__upload">
                  <span className="traffic-ranking__mobile-label" aria-hidden="true">上传</span>
                  {formatBytes(item.upload_bytes)}
                </td>
                <td role="cell" className="traffic-ranking__amount traffic-ranking__download">
                  <span className="traffic-ranking__mobile-label" aria-hidden="true">下载</span>
                  {formatBytes(item.download_bytes)}
                </td>
                <td role="cell" className="traffic-ranking__amount traffic-ranking__total">
                  <span className="traffic-ranking__mobile-label" aria-hidden="true">合计</span>
                  <strong>{formatBytes(item.total_bytes)}</strong>
                  <span className="traffic-ranking__track" aria-hidden="true">
                    <span style={{ width: `${peak ? 100 * item.total_bytes / peak : 0}%` }} />
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
