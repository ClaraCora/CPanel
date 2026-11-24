import { ApiError } from "./auth";
import { sameAgentVersion } from "./resourceModel";
import type { FleetBatch, FleetItem, Machine } from "./types";

const batches = new Map<string, FleetBatch>();
const retriedTasks = new Set<string>();
let latest: FleetBatch | null = null;

function snapshot(batch: FleetBatch, machines: Machine[]) {
  for (const item of batch.items) {
    const machine = machines.find(m => m.id === item.machine_id);
    if (!machine) continue;
    const elapsed = Date.now() - Date.parse(item.updated_at);
    if (item.status === "waiting_online" && machine.status === "online") item.status = "queued";
    else if (item.status === "queued" && elapsed >= 1000) item.status = "dispatched";
    else if (item.status === "dispatched" && elapsed >= 1000) item.status = "acknowledged";
    else if (item.status === "acknowledged" && elapsed >= 1000) {
      if (item.machine_id === "mch_sg02" && !retriedTasks.has(batch.id + item.machine_id)) {
        item.status = "failed";
        item.error = "下载发行文件超时";
      } else {
        item.status = "succeeded";
        machine.agent_version = item.agent_version = batch.target_version;
      }
    } else continue;
    item.updated_at = new Date().toISOString();
  }
  return structuredClone(batch);
}

export function demoFleet(path: string, init: RequestInit, machines: Machine[]) {
  const [, , id, , machineID] = path.split("/");
  if (id === "latest") return latest ? snapshot(latest, machines) : null;
  if (!id && init.method === "POST") {
    const { target_version } = JSON.parse(String(init.body));
    const key = new Headers(init.headers).get("Idempotency-Key") || crypto.randomUUID();
    const existing = batches.get(key);
    if (existing) return snapshot(existing, machines);
    const created_at = new Date().toISOString();
    const items = machines.filter(m => ["pending","online","offline"].includes(m.status)).map((m): FleetItem => ({
      machine_id: m.id, machine_name: m.name, agent_version: m.agent_version, updated_at: created_at,
      status: sameAgentVersion(m.agent_version, target_version) ? "already_current" : m.status === "online" ? "queued" : "waiting_online",
    }));
    latest = { id: key, target_version, created_at, items };
    batches.set(key, latest);
    return snapshot(latest, machines);
  }
  const batch = batches.get(id);
  if (!batch) throw new ApiError(404, {code:"NOT_FOUND", message:"升级任务不存在"});
  if (machineID && init.method === "POST") {
    const item = batch.items.find(i => i.machine_id === machineID);
    if (item && (item.status === "failed" || item.status === "timed_out")) {
      item.status = machines.find(m => m.id === machineID)?.status === "online" ? "queued" : "waiting_online";
      item.updated_at = new Date().toISOString();
      item.error = "";
      retriedTasks.add(batch.id + machineID);
    }
  }
  return snapshot(batch, machines);
}
