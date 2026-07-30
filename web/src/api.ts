import { demoNodes, demoResource, demoSession } from "./mock";
import type { ApiErrorBody, Envelope, Session } from "./types";

const base = "/api/ops/v1";
const requestTimeoutMs = 15_000;
let csrfToken = "";

export const demoMode = new URLSearchParams(window.location.search).get("demo") === "1";

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields: Record<string, string>;

  constructor(status: number, body: ApiErrorBody) {
    super(body.message);
    this.name = "ApiError";
    this.status = status;
    this.code = body.code;
    this.fields = body.fields ?? {};
  }
}

export function setCsrfToken(value: string) {
  csrfToken = value;
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  if (demoMode) return demoRequest<T>(path, init);
  const headers = new Headers(init.headers);
  if (init.body) headers.set("Content-Type", "application/json");
  if (init.method && !["GET", "HEAD"].includes(init.method)) headers.set("X-CSRF-Token", csrfToken);
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), requestTimeoutMs);
  try {
    const response = await fetch(`${base}${path}`, { ...init, headers, credentials: "same-origin", signal: init.signal ?? controller.signal });
    const contentType = response.headers.get("content-type") ?? "";
    if (!contentType.includes("application/json")) {
      throw new ApiError(response.status, { code: "INVALID_RESPONSE", message: response.ok ? "服务返回了无法识别的响应" : `服务请求失败（${response.status}）` });
    }
    const payload = (await response.json()) as Envelope<T>;
    if (!response.ok || payload.error) {
      throw new ApiError(response.status, payload.error ?? { code: "REQUEST_FAILED", message: "请求失败，请稍后重试" });
    }
    return payload.data;
  } catch (reason) {
    if (reason instanceof DOMException && reason.name === "AbortError") {
      throw new ApiError(408, { code: "REQUEST_TIMEOUT", message: "请求超时，请检查网络后重试" });
    }
    throw reason;
  } finally {
    window.clearTimeout(timeout);
  }
}

async function demoRequest<T>(path: string, init: RequestInit): Promise<T> {
  await new Promise((resolve) => window.setTimeout(resolve, 120));
  const method = init.method ?? "GET";
  if (path === "/sessions" && method === "POST") return demoSession as T;
  if (path === "/sessions/current" && method === "DELETE") return { logged_out: true } as T;
  if (path === "/account/profile" && method === "PATCH") {
    const input = JSON.parse(String(init.body));
    demoSession.admin = { ...demoSession.admin, ...input };
    return demoSession.admin as T;
  }
  if (path === "/account/password" && method === "PATCH") return { password_updated: true, other_sessions_revoked: 0 } as T;
  if (method === "GET") return demoResource(path) as T;
  if (path === "/nodes" && method === "POST") {
    const input = JSON.parse(String(init.body));
    const item = { ...demoNodes[0], ...input, id: `nod_demo_${Date.now()}`, agent_id: Date.now() % 100000, machine_name: "演示服务器", status: "draft", current_revision: 0, applied_revision: 0 };
    demoNodes.unshift(item);
    return item as T;
  }
  if (path.startsWith("/nodes/") && method === "PATCH") {
    const id = path.split("/")[2];
    const index = demoNodes.findIndex((item) => item.id === id);
    const input = JSON.parse(String(init.body));
    demoNodes[index] = { ...demoNodes[index], ...input, status: "draft", updated_at: new Date().toISOString() };
    return demoNodes[index] as T;
  }
  if (path.endsWith("/publish") && method === "POST") {
    const id = path.split("/")[2];
    const item = demoNodes.find((node) => node.id === id)!;
    item.status = "published";
    item.current_revision += 1;
    return item as T;
  }
  const resource = path.split("/")[1];
  const input = init.body ? JSON.parse(String(init.body)) : {};
  return { ...input, id: `${resource}_${Date.now()}`, status: "active", created_at: new Date().toISOString(), updated_at: new Date().toISOString() } as T;
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) => request<T>(path, { method: "POST", body: body === undefined ? undefined : JSON.stringify(body) }),
  patch: <T>(path: string, body: unknown) => request<T>(path, { method: "PATCH", body: JSON.stringify(body) }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
  login: async (email: string, password: string) => {
    const session = await request<Session>("/sessions", { method: "POST", body: JSON.stringify({ email, password }) });
    setCsrfToken(session.csrf_token);
    return session;
  },
};
