import { ApiError, demoMode } from "./auth";
import type { Envelope, PortalDashboard, PortalSession } from "./types";

const base = "/ca/edu";
const requestTimeoutMs = 15_000;
let csrfToken = "";
const demoReadOnly = demoMode && new URLSearchParams(window.location.search).get("readonly") === "1";

const demoSession: PortalSession = {
  user: { id: "usr_1001", name: "li.ming", email: "li.ming@example.com", role: "user", status: "active" },
  csrf_token: "csrf_portal_demo",
  expires_at: new Date(Date.now() + 86_400_000).toISOString(),
  read_only: false,
};

const demoDashboard: PortalDashboard = {
  id: "usr_1001", name: "li.ming", role: "user", email: "li.ming@example.com", plan_name: "标准套餐", status: "active",
  traffic_used_bytes: 1_792_000_000, traffic_limit_bytes: 1_099_511_627_776, traffic_reset_at: "2026-09-01T00:00:00+08:00", expires_at: "2030-12-31T16:00:00Z",
  speed_limit_mbps: 0, device_limit: 0, subscription_url: "https://cpanel.example/ca/x/cps_demo_subscription_token",
  nodes: [
    { name: "Neburst-HK", entry_name: "香港入口", protocol: "vless", status: "online", uri: "vless://demo-user@hk.example.com:443?encryption=none&security=reality&type=tcp#Neburst-HK" },
    { name: "V.PS-JP", entry_name: "东京入口", protocol: "vless", status: "online", uri: "vless://demo-user@jp.example.com:443?encryption=none&security=reality&type=tcp#V.PS-JP" },
    { name: "BAGE-SG", entry_name: "新加坡入口", protocol: "shadowsocks", status: "online", uri: "ss://demo@sg.example.com:443#BAGE-SG" },
  ],
};

function demoPortalSession(): PortalSession {
  return demoReadOnly ? { ...demoSession, read_only: true, delegated_by_name: "演示管理员" } : demoSession;
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  if (demoMode) return demoRequest<T>(path, init);
  const headers = new Headers(init.headers);
  if (init.body) headers.set("Content-Type", "application/json");
  if (init.method && !["GET", "HEAD"].includes(init.method) && csrfToken) headers.set("X-CSRF-Token", csrfToken);
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), requestTimeoutMs);
  try {
    const response = await fetch(`${base}${path}`, { ...init, headers, credentials: "same-origin", signal: init.signal ?? controller.signal });
    const contentType = response.headers.get("content-type") ?? "";
    if (!contentType.includes("application/json")) {
      throw new ApiError(response.status, { code: "INVALID_RESPONSE", message: response.ok ? "服务返回了无法识别的响应" : `服务请求失败（${response.status}）` });
    }
    const payload = await response.json() as Envelope<T>;
    if (!response.ok || payload.error) throw new ApiError(response.status, payload.error ?? { code: "REQUEST_FAILED", message: "请求失败，请稍后重试" });
    return payload.data;
  } catch (reason) {
    if (reason instanceof DOMException && reason.name === "AbortError") throw new ApiError(408, { code: "REQUEST_TIMEOUT", message: "请求超时，请检查网络后重试" });
    throw reason;
  } finally {
    window.clearTimeout(timeout);
  }
}

function remember(session: PortalSession) {
  csrfToken = session.csrf_token;
  return session;
}

async function demoRequest<T>(path: string, init: RequestInit): Promise<T> {
  await new Promise((resolve) => window.setTimeout(resolve, 120));
  const method = init.method ?? "GET";
  if (path === "/zl") return demoDashboard as T;
  if (path === "/tc") return { logged_out: true } as T;
  if (path === "/dy" && method === "POST") return { url: `${demoDashboard.subscription_url}-renewed` } as T;
  if (path === "/mm" && method === "PATCH") return { password_updated: true, other_sessions_revoked: 0 } as T;
  return demoPortalSession() as T;
}

export const portalApi = {
  current: () => request<PortalSession>("/hh").then(remember),
  login: (login: string, password: string) => request<PortalSession>("/dl", { method: "POST", body: JSON.stringify({ login, password }) }).then(remember),
  redeem: (grant: string) => request<PortalSession>("/sq", { method: "POST", body: JSON.stringify({ grant }) }).then(remember),
  dashboard: () => request<PortalDashboard>("/zl"),
  changePassword: (currentPassword: string, newPassword: string, confirmPassword: string) => request<{ password_updated: boolean; other_sessions_revoked: number }>("/mm", { method: "PATCH", body: JSON.stringify({ current_password: currentPassword, new_password: newPassword, confirm_password: confirmPassword }) }),
  rotateSubscription: (currentPassword: string) => request<{ url: string }>("/dy", { method: "POST", body: JSON.stringify({ current_password: currentPassword }) }),
  logout: () => request<{ logged_out: boolean }>("/tc", { method: "POST" }),
};
