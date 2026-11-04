import { ApiError } from "./auth";
import type { Resource } from "./resourceDefinitions";
import type { Machine } from "./types";

export function sameFormValues(left: Record<string, string>, right: Record<string, string>) {
  const keys = new Set([...Object.keys(left), ...Object.keys(right)]);
  return [...keys].every((key) => (left[key] ?? "") === (right[key] ?? ""));
}
export const subscriptionAvailable = (item: Resource) => "subscription_available" in item && item.subscription_available === true;
export const resourceName = (item: Resource) => "name" in item && typeof item.name === "string" ? item.name : item.id;
export const statusLabel = (status: string) => ({ active: "启用", online: "在线", published: "已发布", pending: "待接入", draft: "待发布", disabled: "已停用", paused: "已暂停", expired: "已到期", offline: "离线", error: "异常", archived: "已归档" } as Record<string, string>)[status] ?? status;
export function requestErrorMessage(reason: unknown, fallback: string) {
  if (reason instanceof TypeError && /fetch|network|load failed/i.test(reason.message)) return "无法连接服务，请检查网络后重试；当前输入仍已保留";
  if (!(reason instanceof ApiError)) return reason instanceof Error ? reason.message : fallback;
  if (reason.status === 401 || reason.code === "UNAUTHORIZED") return "登录状态已失效，请重新登录";
  if (reason.status === 403 || reason.code === "FORBIDDEN") return "当前账号没有执行此操作的权限";
  if (reason.status === 409 || reason.code === "RESOURCE_CONFLICT") return reason.message || "资源仍被其他配置引用，请先解除关联";
  if (reason.status === 422 || reason.code === "VALIDATION_ERROR") return reason.message || "请检查表单中的字段";
  if (reason.status === 408 || reason.code === "REQUEST_TIMEOUT") return "请求超时，请检查网络后重试";
  return reason.message || fallback;
}
type AgentUpgradeState = "disconnected" | "pending" | "running" | "latest" | "available" | "unknown";
export const latestAgentVersion = (item: Resource) => "latest_agent_version" in item && typeof item.latest_agent_version === "string" ? item.latest_agent_version : "";
export const agentUpgradeState = (item: Resource): AgentUpgradeState => {
  if (!("last_heartbeat_at" in item) || !item.last_heartbeat_at) return "disconnected";
  const status = "agent_upgrade_status" in item ? item.agent_upgrade_status : "";
  if ("agent_upgrade_status" in item) {
    const status = item.agent_upgrade_status;
    if (status === "queued") return "pending";
    if (status === "dispatched" || status === "acknowledged") return "running";
  }
  if (!status && "agent_upgrade_task_id" in item && item.agent_upgrade_task_id) return "agent_upgrade_dispatched_at" in item && item.agent_upgrade_dispatched_at ? "running" : "pending";
  const latest = latestAgentVersion(item);
  if (!latest || !("agent_version" in item) || typeof item.agent_version !== "string") return "unknown";
  return item.agent_version.replace(/^v/i, "").toLowerCase() === latest.replace(/^v/i, "").toLowerCase() ? "latest" : "available";
};
export const canRequestUpgrade = (item: Resource) => agentUpgradeState(item) === "available";
export const upgradeStateLabel = (item: Resource) => {
  if (agentUpgradeState(item) === "available") return `更新至 ${latestAgentVersion(item)}`;
  if ("agent_upgrade_status" in item && item.agent_upgrade_status) {
    const labels: Record<string, string> = { queued: "等待下发", dispatched: "已下发，等待 Agent 确认", acknowledged: "Agent 已确认，正在升级", succeeded: "升级成功", failed: "升级失败", timed_out: "升级超时" };
    const label = labels[String(item.agent_upgrade_status)] || "升级状态未知";
    return label + (item.agent_upgrade_status === "failed" && "agent_upgrade_error" in item && item.agent_upgrade_error ? `：${item.agent_upgrade_error}` : "");
  }
  return ({ disconnected: "Agent 尚未连接", pending: "等待下发", running: "已下发，等待 Agent 确认", latest: "Agent 已是最新版本", available: `更新至 ${latestAgentVersion(item)}`, unknown: "暂时无法检查最新版本" })[agentUpgradeState(item)];
};
export async function writeClipboard(value: string) {
  if (navigator.clipboard?.writeText) {
    try { await navigator.clipboard.writeText(value); return; } catch { /* HTTP deployments use the fallback below. */ }
  }
  const input = document.createElement("textarea");
  input.value = value;
  input.style.position = "fixed";
  input.style.opacity = "0";
  document.body.appendChild(input);
  input.select();
  const copied = document.execCommand("copy");
  input.remove();
  if (!copied) throw new Error("clipboard unavailable");
}
export const localDateTime = (value: string | null) => value ? new Date(new Date(value).getTime() - new Date(value).getTimezoneOffset() * 60000).toISOString().slice(0, 16) : "";


export function agentUpgradeLabel(machine: Machine) {
  const state = agentUpgradeState(machine);
  if (state === "available") return `可更新至 ${machine.latest_agent_version}${machine.agent_upgrade_status === "failed" ? " · 上次升级失败" : machine.agent_upgrade_status === "timed_out" ? " · 上次升级超时" : ""}`;
  if (machine.agent_upgrade_status === "failed") return `上次升级失败${machine.agent_upgrade_error ? `：${machine.agent_upgrade_error}` : ""}`;
  if (machine.agent_upgrade_status === "timed_out") return "上次升级超时，可重新下发";
  if (machine.agent_upgrade_status === "succeeded") return `已升级至 ${machine.agent_version || machine.agent_upgrade_target_version || "最新版本"}`;
  return ({ disconnected: "尚未连接", pending: "升级任务等待心跳", running: "升级执行中", latest: "已是最新版本", unknown: "版本检查暂不可用" } as const)[state];
}

