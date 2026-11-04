import type { ReactNode } from "react";
import { Link } from "wouter";
import type { Machine } from "../types";
import { MachineMetrics } from "./MachineMetrics";
import { formatPreciseDate, StatusBadge } from "./ui";

export function MachineCard({ machine, hideAddress, upgradeLabel, onEdit, actions }: {
  machine: Machine;
  hideAddress: boolean;
  upgradeLabel: string;
  onEdit: () => void;
  actions: ReactNode;
}) {
  return <article className="machine-card" data-resource-id={machine.id}>
    <header>
      <button type="button" className="machine-card__name" onClick={onEdit}>{machine.name}</button>
      <StatusBadge status={machine.status} />
    </header>
    <div className="machine-card__identity">
      <p className="machine-card__address mono">{hideAddress ? "地址已隐藏" : machine.host || "未设置地址"}</p>
      <p className="machine-card__region">{machine.region || "未设置区域"}</p>
    </div>
    <MachineMetrics machine={machine} />
    <div className="machine-card__agent">
      <span><b>Agent</b><span className="mono">{machine.agent_version || "未接入"}</span><span className="machine-card__protocol">{machine.agent_protocol === "v2" ? "V2 加密" : "旧通讯"}</span></span>
      <small>{upgradeLabel}</small>
    </div>
    <dl className="machine-card__timestamps">
      <div><dt>最后心跳</dt><dd>{formatPreciseDate(machine.last_heartbeat_at)}</dd></div>
      <div><dt>指标采样</dt><dd>{formatPreciseDate(machine.metrics_sampled_at)}</dd></div>
    </dl>
    <footer>
      <Link to={`/nodes?machine=${encodeURIComponent(machine.id)}`}>{machine.node_count} 个节点</Link>
      {actions}
    </footer>
  </article>;
}
