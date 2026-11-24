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
  return <article className="machine-card" data-resource-id={machine.id} aria-label={machine.name}>
    <header className="machine-card__identity">
      <div className="machine-card__title">
        <h3><button type="button" className="machine-card__name" onClick={onEdit}>{machine.name}</button></h3>
        <StatusBadge status={machine.status} />
      </div>
      <p className="machine-card__address mono">{hideAddress ? "地址已隐藏" : machine.host || "未设置地址"}</p>
      <p className="machine-card__location">
        <span>{machine.region || "未设置区域"}</span>
        <Link to={`/nodes?machine=${encodeURIComponent(machine.id)}`}>{machine.node_count} 个节点</Link>
      </p>
    </header>
    <MachineMetrics machine={machine} />
    <dl className="machine-card__details">
      <div><dt>Agent 版本</dt><dd><span className="mono">{machine.agent_version || "未接入"}</span><span className="machine-card__protocol">{machine.agent_protocol === "v2" ? "V2 加密" : "旧通讯"}</span></dd></div>
      <div><dt>升级状态</dt><dd>{upgradeLabel}</dd></div>
      <div><dt>最后心跳</dt><dd>{formatPreciseDate(machine.last_heartbeat_at)}</dd></div>
    </dl>
    <footer>{actions}</footer>
  </article>;
}
