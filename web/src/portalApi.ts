import { ApiError, demoMode, notifyPortalSessionExpired } from "./auth";
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

function demoPortalSession(): PortalSession {
  return demoReadOnly ? { ...demoSession, read_only: true, delegated_by_name: "演示管理员" } : demoSession;
}

async function request<T>(path: string, init: RequestInit = {}, requiresSession = false): Promise<T> {
  if (demoMode) return demoRequest<T>(path, init);
  const headers = new Headers(init.headers);
  if (init.body) headers.set("Content-Type", "application/json");
  if (init.method && !["GET", "HEAD"].includes(init.method) && csrfToken) headers.set("X-CSRF-Token", csrfToken);
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), requestTimeoutMs);
  try {
    const response = await fetch(`${base}${path}`, { ...init, headers, credentials: "same-origin", signal: init.signal ?? controller.signal });
    if (requiresSession && response.status === 401) expirePortalSession();
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

function expirePortalSession() {
  csrfToken = "";
  notifyPortalSessionExpired();
}

async function demoRequest<T>(path: string, init: RequestInit): Promise<T> {
  await new Promise((resolve) => window.setTimeout(resolve, 120));
  const method = init.method ?? "GET";
  if (path === "/zl") {
    const { demoPortalDashboard } = await import("./PortalDashboardPage");
    return demoPortalDashboard as T;
  }
  if (path === "/tc") return { logged_out: true } as T;
  if (path === "/dy" && method === "POST") return { url: "https://example.invalid/ca/x/demo-renewed" } as T;
  if (path === "/mm" && method === "PATCH") return { password_updated: true, other_sessions_revoked: 0 } as T;
  return demoPortalSession() as T;
}

export const portalApi = {
  current: () => request<PortalSession>("/hh").then(remember),
  login: (login: string, password: string) => request<PortalSession>("/dl", { method: "POST", body: JSON.stringify({ login, password }) }).then(remember),
  redeem: (grant: string) => request<PortalSession>("/sq", { method: "POST", body: JSON.stringify({ grant }) }).then(remember),
  dashboard: () => request<PortalDashboard>("/zl", {}, true),
  changePassword: (currentPassword: string, newPassword: string, confirmPassword: string) => request<{ password_updated: boolean; other_sessions_revoked: number }>("/mm", { method: "PATCH", body: JSON.stringify({ current_password: currentPassword, new_password: newPassword, confirm_password: confirmPassword }) }, true),
  rotateSubscription: (currentPassword: string) => request<{ url: string }>("/dy", { method: "POST", body: JSON.stringify({ current_password: currentPassword }) }, true),
  logout: () => request<{ logged_out: boolean }>("/tc", { method: "POST" }, true),
};
