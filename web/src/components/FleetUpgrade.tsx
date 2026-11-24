import { useRef, useState } from "react";
import { History, RefreshCw, Upload } from "lucide-react";
import { api, ApiError } from "../api";
import { useResource } from "../hooks";
import { creationIntent } from "../mutations";
import { sameAgentVersion } from "../resourceModel";
import type { FleetBatch, FleetItem, FleetStatus, Machine } from "../types";
import { Button, DetailDialog, formatPreciseDate, useToast } from "./ui";

const path = "/agent-upgrade-batches";
const labels: Record<FleetStatus, string> = {
  waiting_online: "待上线", queued: "等待下发", dispatched: "已下发", acknowledged: "正在升级",
  succeeded: "升级成功", already_current: "已是目标版本", failed: "升级失败", timed_out: "升级超时",
};
function achieved(status: FleetStatus) { return status === "succeeded" || status === "already_current"; }
function retryable(status: FleetStatus) { return status === "failed" || status === "timed_out"; }
type Props = { machines: Machine[]; ready: boolean; refreshMachines: () => Promise<void> };
type Plan = { target: string; machines: Machine[] };

export function FleetUpgrade({ machines, ready, refreshMachines }: Props) {
  const latest = useResource<FleetBatch | null>(path + "/latest", null, 30_000);
  const [plan, setPlan] = useState<Plan | null>(null);
  const [view, setView] = useState<FleetBatch | null>(null);
  const [busy, setBusy] = useState(false);
  const lock = useRef(false);
  const intent = useRef(creationIntent());
  const toast = useToast();
  const eligible = machines.filter(m => ["online", "offline", "pending"].includes(m.status));
  const target = eligible.find(m => m.latest_agent_version)?.latest_agent_version || "";
  function openPlan() {
    intent.current = creationIntent();
    setPlan({ machines: eligible, target });
  }
  async function submit() {
    if (!plan || !plan.target || lock.current) return;
    lock.current = true;
    setBusy(true);
    try {
      const payload = { target_version: plan.target };
      const batch = await api.post<FleetBatch>(path, payload, { idempotencyKey: intent.current(payload) });
      setView(batch);
      setPlan(null);
      latest.setData(batch);
      void latest.reload(true);
      void refreshMachines();
    } catch (err) {
      if (err instanceof ApiError && err.code === "AGENT_RELEASE_CHANGED") {
        setPlan(null);
        void refreshMachines();
      }
      toast(err instanceof Error ? err : "升级任务创建失败", "error");
    } finally { lock.current = false; setBusy(false); }
  }
  return <>
    {latest.data?.id && <Button type="button" onClick={() => setView(latest.data)}><History size={16} />最近升级</Button>}
    {latest.error && <Button type="button" onClick={() => void latest.reload()}>重新加载升级记录</Button>}
    <Button type="button" disabled={!ready || !eligible.length} onClick={openPlan}><Upload size={16} />升级全部 Agent</Button>
    <DetailDialog open={Boolean(plan)} title="确认升级全部 Agent" className="fleet-dialog" onClose={() => { if (!busy) setPlan(null); }}>
      {plan && <><p>全部 {plan.machines.length} 台服务器加入任务。在线机器在下次心跳领取升级，离线机器重新连接后自动继续。</p>
        <div className="fleet-target"><span>目标版本</span><strong className="mono">{plan.target || "暂时无法获取"}</strong></div>
        <ul className="fleet-items">{plan.machines.map(m => <li key={m.id}><div><strong>{m.name}</strong><small>{m.agent_version || "未接入"}</small></div><span>{sameAgentVersion(m.agent_version, plan.target) ? "已是目标版本" : m.status === "online" ? "在线待升级" : "待上线"}</span></li>)}</ul>
        <footer className="fleet-actions"><Button disabled={busy} onClick={() => setPlan(null)}>取消</Button>{plan.target
          ? <Button variant="primary" loading={busy} onClick={() => void submit()}>升级全部 {plan.machines.length} 台</Button>
          : <Button onClick={() => { setPlan(null); void refreshMachines(); }}>重新检查版本</Button>}</footer></>}
    </DetailDialog>
    {view && <FleetProgress key={view.id} initial={view} onClose={() => { setView(null); void latest.reload(true); void refreshMachines(); }} />}
  </>;
}

function FleetProgress({ initial, onClose }: { initial: FleetBatch; onClose: () => void }) {
  const endpoint = path + "/" + encodeURIComponent(initial.id);
  const batch = useResource<FleetBatch>(endpoint, initial, 5_000);
  const [pending, setPending] = useState("");
  const lock = useRef(false);
  const retryKeys = useRef(new Map<string, ReturnType<typeof creationIntent>>());
  const toast = useToast();
  const current = batch.data || initial;
  const items = current.items;
  const completed = items.filter(i => achieved(i.status)).length;
  const waiting = items.filter(i => i.status === "waiting_online").length;
  const failed = items.filter(i => retryable(i.status)).length;
  async function retry(item: FleetItem) {
    if (lock.current) return;
    lock.current = true;
    setPending(item.machine_id);
    const getKey = retryKeys.current.get(item.machine_id) || creationIntent();
    retryKeys.current.set(item.machine_id, getKey);
    try {
      const result = await api.post<FleetBatch>(endpoint + "/machines/" + encodeURIComponent(item.machine_id) + "/retry", {}, { idempotencyKey: getKey([item.task_id, item.updated_at]) });
      batch.setData(result);
      await batch.reload();
      retryKeys.current.delete(item.machine_id);
    } catch (err) { toast(err instanceof Error ? err : "重试失败", "error"); void batch.reload(true); }
    finally { lock.current = false; setPending(""); }
  }
  return <DetailDialog open title="升级进度" className="fleet-dialog" onClose={onClose}>
    <div className="fleet-target"><span>目标版本</span><strong className="mono">{current.target_version}</strong></div>
    <div className="fleet-counts" aria-live="polite"><strong>{completed} / {items.length} 已达目标</strong><span>{failed} 台失败 · {waiting} 台待上线</span></div>
    <progress max={Math.max(items.length, 1)} value={completed} aria-label="已达目标版本的机器比例" />
    {(batch.error || batch.refreshError) && <div role="alert" className="form-alert">{batch.error || batch.refreshError} <Button onClick={() => void batch.reload()}>重新加载</Button></div>}
    <ul className="fleet-items">{items.map(item => <li key={item.machine_id}><div><strong>{item.machine_name}</strong><small>{item.agent_version || "未接入"} · {formatPreciseDate(item.updated_at)}</small>{item.error && <small className="fleet-error">{item.error}</small>}</div>
      <span className={retryable(item.status) ? "fleet-error" : achieved(item.status) ? "text-success" : ""}>{labels[item.status] || item.status}</span>
      {retryable(item.status) && <Button loading={pending === item.machine_id} disabled={Boolean(pending)} aria-label={`重试 ${item.machine_name}`} onClick={() => void retry(item)}><RefreshCw size={14} />重试</Button>}</li>)}</ul>
    <footer className="fleet-actions"><Button onClick={onClose}>关闭</Button></footer>
  </DetailDialog>;
}
