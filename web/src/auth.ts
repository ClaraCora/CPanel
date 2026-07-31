import type { ApiErrorBody, Envelope, Session } from "./types";

const base = "/ca/ht";
const requestTimeoutMs = 15_000;
let csrfToken = "";

export const demoMode = import.meta.env.DEV && new URLSearchParams(window.location.search).get("demo") === "1";

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

export function getCsrfToken() {
  return csrfToken;
}

function setCsrfToken(value: string) {
  csrfToken = value;
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
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

async function storeSession(session: Session) {
  setCsrfToken(session.csrf_token);
  return session;
}

export const authApi = {
  current: async () => storeSession(await request<Session>("/hh")),
  login: async (email: string, password: string) => {
    if (demoMode) {
      const { demoSession } = await import("./mock");
      return storeSession(demoSession);
    }
    return storeSession(await request<Session>("/hh", { method: "POST", body: JSON.stringify({ email, password }) }));
  },
  logout: () => demoMode ? Promise.resolve({ logged_out: true }) : request<{ logged_out: boolean }>("/hh/dq", { method: "DELETE" }),
};
