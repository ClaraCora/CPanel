import type { Machine } from "../types";
import { formatPreciseDate } from "./ui";

export function MachineMetrics({ machine }: { machine: Machine }) {
  const memory = usagePercent(machine.metrics?.mem?.used, machine.metrics?.mem?.total);
  const disk = usagePercent(machine.metrics?.disk?.used, machine.metrics?.disk?.total);
  return <div className="machine-metrics" title={`采样时间 ${formatPreciseDate(machine.metrics_sampled_at)}`}>
    {[["CPU", machine.metrics_sampled_at ? machine.metrics?.cpu : undefined], ["内存", machine.metrics_sampled_at ? memory : undefined], ["磁盘", machine.metrics_sampled_at ? disk : undefined]].map(([label, value]) => <span className="machine-metric" key={label}><b>{label}</b><i className="machine-metric__track" aria-hidden="true"><i style={{ width: `${typeof value === "number" && Number.isFinite(value) ? Math.max(0, Math.min(100, value)) : 0}%` }} /></i><span>{formatPercent(typeof value === "number" ? value : undefined)}</span></span>)}
  </div>;
}

function usagePercent(used?: number, total?: number) {
  return typeof used === "number" && typeof total === "number" && total > 0 ? used / total * 100 : undefined;
}

function formatPercent(value?: number) {
  return typeof value === "number" && Number.isFinite(value) ? `${value.toLocaleString("zh-CN", { maximumFractionDigits: 1 })}%` : "—";
}

