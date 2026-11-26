import { ArrowDown, ArrowUp, Boxes, CalendarDays, ChevronDown, Server, UsersRound } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useState } from "react";
import type { Overview } from "../types";
import { formatBytes } from "./ui";

type OverviewMetric = {
  label: string;
  icon: LucideIcon;
  total: number;
  initiallyOpen?: boolean;
  details: { label: string; value: number; tone?: "online" | "offline" }[];
};

export function OverviewTraffic({ data }: { data: Overview }) {
  const trafficDay = data.traffic_day?.slice(0, 10);
  const end = trafficDay ? Date.parse(`${trafficDay}T00:00:00Z`) : NaN;
  const samples = new Map(data.traffic_week?.map((row) => [row.day.slice(0, 10), row]));
  const days = Number.isFinite(end) ? Array.from({ length: 7 }, (_, i) => {
    const day = new Date(end - (6 - i) * 86_400_000).toISOString().slice(0, 10);
    const sample = samples.get(day);
    return { day, sample, total: sample ? sample.upload_bytes + sample.download_bytes : 0 };
  }) : [];
  const [selectedDay, setSelectedDay] = useState("");
  const selected = days.find(({ day }) => day === selectedDay && day !== trafficDay);
  const shownDate = selected?.day ?? trafficDay;
  const upload = selected ? selected.sample?.upload_bytes : data.traffic_today_upload_bytes;
  const download = selected ? selected.sample?.download_bytes : data.traffic_today_download_bytes;
  const total = selected ? selected.sample && selected.total : data.traffic_today_bytes;
  const peak = Math.max(0, ...days.map(({ total }) => total));
  const metrics: OverviewMetric[] = [
    {
      label: "服务器",
      icon: Server,
      total: data.machines_total,
      initiallyOpen: true,
      details: [
        { label: "在线", value: data.machines_online, tone: "online" },
        { label: "离线", value: data.machines_offline, tone: data.machines_offline > 0 ? "offline" : undefined },
      ],
    },
    {
      label: "节点",
      icon: Boxes,
      total: data.nodes_total,
      details: [{ label: "已发布", value: data.nodes_published }],
    },
    {
      label: "订阅账号",
      icon: UsersRound,
      total: data.admins_active + data.users_active + data.friends_active,
      details: [
        { label: "管理员", value: data.admins_active },
        { label: "用户", value: data.users_active },
        { label: "朋友", value: data.friends_active },
      ],
    },
  ];

  return (
    <section className="overview-report" aria-label="流量与运行指标">
      <div className="traffic-report">
        <header className="traffic-report__header">
          <h2>每日流量</h2>
          <span className="traffic-report__date">
            <CalendarDays size={15} aria-hidden="true" />
            {shownDate ? <time dateTime={shownDate}>{shownDate}</time> : "今日"}
          </span>
        </header>
        <dl className="traffic-report__summary" aria-live="polite">
          <div className="traffic-report__total">
            <dt>合计流量</dt>
            <dd>{total === undefined ? "无采样" : formatBytes(total)}</dd>
          </div>
          <div className="traffic-report__transfer">
            <div className="traffic-report__upload">
              <dt><ArrowUp size={14} aria-hidden="true" />{selected ? "上传" : "今日上传"}</dt>
              <dd>{upload === undefined ? "—" : formatBytes(upload)}</dd>
            </div>
            <div className="traffic-report__download">
              <dt><ArrowDown size={14} aria-hidden="true" />{selected ? "下载" : "今日下载"}</dt>
              <dd>{download === undefined ? "—" : formatBytes(download)}</dd>
            </div>
          </div>
        </dl>
        {days.length > 0 ? (
          <figure className="traffic-chart" aria-label="近七日上传与下载流量">
            <div className="traffic-chart__axis" aria-hidden="true">
              <span>{peak > 0 && formatBytes(peak)}</span>
              <span>{peak >= 2 && formatBytes(peak / 2)}</span>
              <span>0 B</span>
            </div>
            <div className="traffic-chart__days">
              {days.map(({ day, total, sample }) => {
                const label = sample
                  ? `${day} 上传 ${formatBytes(sample.upload_bytes)}，下载 ${formatBytes(sample.download_bytes)}`
                  : `${day} 无采样`;
                return (
                  <button
                    type="button"
                    className="traffic-chart__day"
                    key={day}
                    aria-pressed={day === shownDate}
                    aria-label={label}
                    title={label}
                    onClick={() => setSelectedDay(day)}
                  >
                    <span className="traffic-chart__plot" aria-hidden="true">
                      {sample && total > 0 ? (
                        <span className="traffic-chart__stack" style={{ height: `${100 * total / peak}%` }}>
                          <i style={{ flexGrow: sample.upload_bytes }} />
                          <i style={{ flexGrow: sample.download_bytes }} />
                        </span>
                      ) : <span className="traffic-chart__empty-mark">{sample ? "0" : "—"}</span>}
                    </span>
                    <span className="traffic-chart__date">{Number(day.slice(5, 7))}/{Number(day.slice(8, 10))}</span>
                  </button>
                );
              })}
            </div>
            <figcaption className="traffic-chart__legend">
              <span className="traffic-report__upload">上传</span>
              <span className="traffic-report__download">下载</span>
              <span>— 无采样</span>
            </figcaption>
          </figure>
        ) : <p className="traffic-chart__empty">暂无近七日流量数据</p>}
      </div>
      <div className="overview-index">
        <h2>运行指标</h2>
        <div className="overview-index__rows">
          {metrics.map(({ label, icon: Icon, total, initiallyOpen, details }) => (
            <details key={label} open={initiallyOpen}>
              <summary>
                <Icon size={18} className="overview-index__icon" aria-hidden="true" />
                <span>{label}</span>
                <strong>{total}</strong>
                <ChevronDown size={16} className="overview-index__chevron" aria-hidden="true" />
              </summary>
              <dl className="overview-index__breakdown">
                {details.map(({ label, value, tone }) => (
                  <div key={label} data-tone={tone}>
                    <dt>{label}</dt>
                    <dd>{value}</dd>
                  </div>
                ))}
              </dl>
            </details>
          ))}
        </div>
      </div>
    </section>
  );
}
