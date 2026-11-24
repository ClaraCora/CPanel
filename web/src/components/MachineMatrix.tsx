import type { ReactNode } from "react";
import { Link } from "wouter";
import { formatPreciseDate, StatusBadge } from "./ui";
import { machineMetricValues, MetricValue } from "./MachineMetrics";
import { agentUpgradeLabel } from "../resourceModel";
import type { Machine } from "../types";

type Props = { machines: Machine[]; hideAddress: boolean; onEdit: (machine: Machine) => void; actions: (machine: Machine) => ReactNode };

export function MachineMatrix({ machines, hideAddress, onEdit, actions }: Props) {
  const rows: [string, (machine: Machine) => ReactNode][] = [
    ["运行状态", m => <StatusBadge status={m.status} />],
    ...(["CPU", "内存", "磁盘"] as const).map((label, i): [string, (m: Machine) => ReactNode] =>
      [label, m => <span className="matrix-metric" title={`采样时间 ${formatPreciseDate(m.metrics_sampled_at)}`}><MetricValue value={machineMetricValues(m)[i]} /></span>]),
    ["Agent 版本", m => <><span className="mono">{m.agent_version || "未接入"}</span><small>{m.agent_protocol === "v2" ? "V2 加密" : "旧通讯"}</small></>],
    ["升级状态", m => agentUpgradeLabel(m)],
    ["区域 / 节点", m => <>{m.region || "未设置区域"} · <Link to={`/nodes?machine=${encodeURIComponent(m.id)}`}>{m.node_count} 个节点</Link></>],
    ["最后心跳", m => formatPreciseDate(m.last_heartbeat_at)],
    ["操作", actions],
  ];
  return <section className="machine-comparison"><header><h2>服务器指标对照</h2><span>{machines.length} 台服务器</span></header>
    <div className="machine-comparison__scroll" tabIndex={0} role="region" aria-label="可横向滚动的服务器指标对照">
      <table aria-label="服务器指标对照"><thead><tr><th scope="col">指标</th>{machines.map(m =>
        <th key={m.id} scope="col" data-resource-id={m.id}><button type="button" onClick={() => onEdit(m)}>{m.name}</button><small className="mono">{hideAddress ? "地址已隐藏" : m.host || "未设置地址"}</small></th>)}</tr></thead>
        <tbody>{rows.map(([label, cell]) => <tr key={label}><th scope="row">{label}</th>{machines.map(m => <td key={m.id} data-resource-id={label === "操作" ? m.id : undefined}>{cell(m)}</td>)}</tr>)}</tbody>
      </table>
    </div></section>;
}

export function MachineTable({ machines, hideAddress, onEdit, actions }: Props) {
  return <section className="machine-comparison machine-row-list">
    <header><h2>服务器清单</h2><span>{machines.length} 台服务器</span></header>
    <div className="machine-comparison__scroll" tabIndex={0} role="region" aria-label="可横向滚动的服务器清单">
      <table aria-label="服务器列表"><thead><tr>{["服务器","状态","CPU","内存","磁盘","Agent 版本","升级状态","操作"].map(label => <th scope="col" key={label}>{label}</th>)}</tr></thead>
        <tbody>{machines.map(m => <tr key={m.id} data-resource-id={m.id}>
          <th scope="row"><button type="button" aria-label={`查看并编辑 ${m.name}`} onClick={() => onEdit(m)}>{m.name}</button><small className="mono">{hideAddress ? "地址已隐藏" : m.host || "未设置地址"}</small><small>{m.region || "未设置区域"} · <Link to={`/nodes?machine=${encodeURIComponent(m.id)}`}>{m.node_count} 个节点</Link></small></th>
          <td><StatusBadge status={m.status} /></td>
          {machineMetricValues(m).map((value,i) => <td key={i}><span className="matrix-metric" title={`采样时间 ${formatPreciseDate(m.metrics_sampled_at)}`}><MetricValue value={value} /></span></td>)}
          <td><span className="mono">{m.agent_version || "未接入"}</span><small>{m.agent_protocol === "v2" ? "V2 加密" : "旧通讯"}</small></td>
          <td>{agentUpgradeLabel(m)}<small>最后心跳 {formatPreciseDate(m.last_heartbeat_at)}</small></td>
          <td>{actions(m)}</td>
        </tr>)}</tbody>
      </table>
    </div>
  </section>;
}
