import { useState } from "react";
import type { Overview } from "../types";
import { formatBytes } from "./ui";

export function OverviewTraffic({ data }: { data: Overview }) {
  const end = data.traffic_day && Date.parse(data.traffic_day + "T00:00:00Z");
  const samples = new Map(data.traffic_week?.map((row) => [row.day.slice(0, 10), row]));
  const days = typeof end === "number" && Number.isFinite(end) ? Array.from({ length: 7 }, (_, i) => {
    const day = new Date(end - (6 - i) * 86_400_000).toISOString().slice(0, 10);
    const sample = samples.get(day);
    return { day, sample, total: sample ? sample.upload_bytes + sample.download_bytes : 0 };
  }) : [];
  const [selectedDay, setSelectedDay] = useState("");
  const selected = days.find(({ day }) => day === selectedDay && day !== data.traffic_day);
  const shownDate = selected?.day ?? data.traffic_day;
  const upload = selected ? selected.sample?.upload_bytes : data.traffic_today_upload_bytes;
  const download = selected ? selected.sample?.download_bytes : data.traffic_today_download_bytes;
  const total = selected ? selected.sample && selected.total : data.traffic_today_bytes;
  const peak = Math.max(0, ...days.map(({ total }) => total));
  return <section className="overview-report" aria-label="流量与运行指标">
    <div className="traffic-report">
      <h2>每日流量</h2>
      <p>{shownDate ?? "今日"}</p>
      <strong className="traffic-report__total">{total === undefined ? "无采样" : formatBytes(total)}</strong>
      {days.length > 0 ? <figure className="traffic-chart" aria-label="近七日上传与下载流量">
        <div className="traffic-chart__axis" aria-hidden="true"><span>{peak ? formatBytes(peak) : ""}</span><span>0 B</span></div>
        <div className="traffic-chart__days">{days.map(({ day, total, sample }) => {
          const label = sample ? `${day} 上传 ${formatBytes(sample.upload_bytes)}，下载 ${formatBytes(sample.download_bytes)}` : `${day} 无采样`;
          return <button type="button" className="traffic-chart__day" key={day} aria-pressed={day === shownDate} aria-label={label} title={label} onClick={() => setSelectedDay(day)}>
            <span className="traffic-chart__plot">{sample ? total > 0 ? <span className="traffic-chart__stack" style={{ height: `${100 * total / peak}%` }}>
              <i style={{ flexGrow: sample.upload_bytes }} /><i style={{ flexGrow: sample.download_bytes }} />
            </span> : <span>0</span> : <span>—</span>}</span>
            <small>{Number(day.slice(5, 7))}/{Number(day.slice(8, 10))}</small>
          </button>;
        })}</div>
      </figure> : <p>暂无近七日流量数据</p>}
      <div className="traffic-chart__legend" aria-live="polite">
        <span>{selected ? "上传" : "今日上传"} {upload === undefined ? "—" : formatBytes(upload)}</span>
        <span>{selected ? "下载" : "今日下载"} {download === undefined ? "—" : formatBytes(download)}</span>
        <small>— 无采样</small>
      </div>
    </div>
    <div className="overview-index"><h2>运行指标</h2>
      <details open><summary>服务器 <b>{data.machines_total}</b></summary><p>{data.machines_online} 在线 / {data.machines_offline} 离线</p></details>
      <details><summary>节点 <b>{data.nodes_total}</b></summary><p>{data.nodes_published} 已发布</p></details>
      <details><summary>订阅账号 <b>{data.admins_active + data.users_active + data.friends_active}</b></summary><p>{data.admins_active} 管理员 / {data.users_active} 用户 / {data.friends_active} 朋友</p></details>
    </div>
  </section>;
}
