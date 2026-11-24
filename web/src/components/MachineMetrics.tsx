import type { Machine } from "../types";
import { formatPreciseDate } from "./ui";

export function MachineMetrics({ machine }: { machine: Machine }) {
  const values = machineMetricValues(machine);
  return <div className="machine-metrics" title={`采样时间 ${formatPreciseDate(machine.metrics_sampled_at)}`}>
    {["CPU", "内存", "磁盘"].map((label, i) => <span className="machine-metric" key={label}><b>{label}</b><MetricValue value={values[i]} /></span>)}
  </div>;
}

export function machineMetricValues(machine: Machine) {
  if (!machine.metrics_sampled_at) return [undefined, undefined, undefined];
  return [machine.metrics?.cpu, usagePercent(machine.metrics?.mem?.used, machine.metrics?.mem?.total), usagePercent(machine.metrics?.disk?.used, machine.metrics?.disk?.total)];
}

export function MetricValue({ value }: { value: number | undefined }) {
  const width = typeof value === "number" && Number.isFinite(value) ? Math.max(0, Math.min(100, value)) : 0;
  return <><i className="machine-metric__track" aria-hidden="true"><i style={{ width: `${width}%` }} /></i><span>{formatPercent(value)}</span></>;
}

function usagePercent(used?: number, total?: number) {
  return typeof used === "number" && typeof total === "number" && total > 0 ? used / total * 100 : undefined;
}

function formatPercent(value?: number) {
  return typeof value === "number" && Number.isFinite(value) ? `${value.toLocaleString("zh-CN", { maximumFractionDigits: 1 })}%` : "—";
}

