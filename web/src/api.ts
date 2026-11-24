import { ApiError, adminSessionExpired, demoMode, getCsrfToken, notifyAdminSessionExpired, trackProtectedRequest } from "./auth";
import { mutationHeaders, type MutationOptions } from "./mutations";
import type { Envelope } from "./types";

export { ApiError, demoMode } from "./auth";

const base = "/ca/ht";
const requestTimeoutMs = 15_000;
const resourcePaths: Record<string, string> = {
  machines: "fwq", nodes: "jd", "access-groups": "qxz", plans: "tc", users: "yh",
  "route-policies": "ly", outbounds: "ck",
};
const settingPaths: Record<string, string> = {
  site: "zd", agent: "dl", security: "aq", node_defaults: "jdmr", certificate: "zs", subscription: "dy", retention: "bl", tgbot: "tg",
};
const rolePaths: Record<string, string> = { admin: "gly", user: "yh", friend: "py" };

function wirePath(path: string): string {
	const queryIndex = path.indexOf("?");
	const query = queryIndex >= 0 ? path.slice(queryIndex) : "";
	const pathname = queryIndex >= 0 ? path.slice(0, queryIndex) : path;
	return `${wirePathname(pathname)}${query}`;
}

function wirePathname(path: string): string {
  const segments = path.split("/").filter(Boolean);
  if (segments.length === 0) return path;
  if (segments[0] === "session") return "/hh";
  if (segments[0] === "sessions") return segments[1] === "current" ? "/hh/dq" : "/hh";
  if (segments[0] === "account") return segments[1] === "profile" ? "/zh/zl" : "/zh/mm";
  if (segments[0] === "tools" && segments[1] === "reality-keypair") return "/gj/xsmy";
  if (segments[0] === "overview") return "/zl";
  if (segments[0] === "history") return "/ls";
  if (segments[0] === "audit-events") return "/sj";
  if (segments[0] === "user-access-ips") return segments[1] === "location" ? "/yh/fwjl/gs" : "/yh/fwjl";
  if (segments[0] === "settings" && segments[1] === "subscription" && segments[2] === "access-log") return "/sz/dy/jl";
  if (segments[0] === "settings" && segments[1] === "tgbot" && segments[2] === "test") return "/sz/tg/cs";
  if (segments[0] === "settings") return `/sz/${settingPaths[segments[1]] ?? segments[1]}`;
  const resource = resourcePaths[segments[0]];
  if (!resource) return path;
  const tail = segments.slice(1);
  if (segments[0] === "machines" && tail[1] === "installation") tail[1] = "az";
  if (segments[0] === "machines" && tail[1] === "agent-upgrade") tail[1] = "sj";
  if (segments[0] === "machines" && tail[1] === "agent-identity") tail[1] = "sf";
  if (segments[0] === "machines" && tail[1] === "credentials") tail[1] = "pz";
  if (segments[0] === "nodes" && tail[1] === "publish") tail[1] = "fb";
  if (segments[0] === "nodes" && tail[1] === "revisions") tail[1] = "bb";
  if (segments[0] === "nodes" && tail[1] === "restore") tail[1] = "hf";
  if (segments[0] === "users" && tail[0] === "quick") {
    tail[0] = "ks";
    tail[1] = rolePaths[tail[1]] ?? tail[1];
  }
  if (segments[0] === "users" && tail[1] === "subscription") tail[1] = "dy";
	if (segments[0] === "users" && tail[1] === "portal") tail[1] = "edu";
  return `/${resource}${tail.length ? `/${tail.join("/")}` : ""}`;
}

async function request<T>(path: string, init: RequestInit = {}, receiveMeta?: (meta: Envelope<T>["meta"]) => void): Promise<T> {
  if (demoMode) { receiveMeta?.({ request_id: "demo", config_version: 1 }); return (await import("./mock")).demoRequest<T>(path, init); }
  if (adminSessionExpired()) throw new ApiError(401, { code: "ADMIN_SESSION_INVALID", message: "请重新登录后继续操作；编辑内容仍保留在当前页面" });
  const headers = new Headers(init.headers);
  if (init.body) headers.set("Content-Type", "application/json");
  if (init.method && !["GET", "HEAD"].includes(init.method)) headers.set("X-CSRF-Token", getCsrfToken());
  const controller = new AbortController();
  const untrack = trackProtectedRequest(controller);
  let timedOut = false;
  const abortFromCaller = () => controller.abort(init.signal?.reason);
  if (init.signal?.aborted) abortFromCaller();
  else init.signal?.addEventListener("abort", abortFromCaller, { once: true });
  const timeout = window.setTimeout(() => {
    timedOut = true;
    controller.abort();
  }, requestTimeoutMs);
  try {
    const response = await fetch(`${base}${wirePath(path)}`, { ...init, headers, credentials: "same-origin", signal: controller.signal });
    if (response.status === 401) {
      // Keep this response readable while cancelling the other session requests.
      untrack();
      notifyAdminSessionExpired();
    }
    const contentType = response.headers.get("content-type") ?? "";
    if (!contentType.includes("application/json")) {
      throw new ApiError(response.status, { code: "INVALID_RESPONSE", message: response.ok ? "服务返回了无法识别的响应" : `服务请求失败（${response.status}）` }, response.headers.get("X-Request-ID") ?? "");
    }
    const payload = (await response.json()) as Envelope<T>;
    receiveMeta?.(payload.meta ?? { request_id: response.headers.get("X-Request-ID") || "" });
    if (!response.ok || payload.error) {
      throw new ApiError(response.status, payload.error ?? { code: "REQUEST_FAILED", message: "请求失败，请稍后重试" }, payload.meta?.request_id || response.headers.get("X-Request-ID") || "");
    }
    return payload.data;
  } catch (reason) {
    if (reason instanceof DOMException && reason.name === "AbortError") {
      if (!timedOut) throw reason;
      throw new ApiError(408, { code: "REQUEST_TIMEOUT", message: "请求超时，请检查网络后重试" });
    }
    throw reason;
  } finally {
    window.clearTimeout(timeout);
    untrack();
    init.signal?.removeEventListener("abort", abortFromCaller);
  }
}

export const api = {
  get: <T>(path: string, signal?: AbortSignal) => request<T>(path, { signal }),
  getWithMeta: async <T>(path: string, signal?: AbortSignal) => {
    let meta: Envelope<T>["meta"] = { request_id: "" };
    const data = await request<T>(path, { signal }, (value) => { meta = value; });
    return { data, meta };
  },
  post: <T>(path: string, body?: unknown, options?: MutationOptions) => request<T>(path, { method: "POST", headers: mutationHeaders(options), body: body === undefined ? undefined : JSON.stringify(body) }),
  patch: <T>(path: string, body: unknown, options?: MutationOptions) => request<T>(path, { method: "PATCH", headers: mutationHeaders(options), body: JSON.stringify(body) }),
  delete: <T>(path: string, options?: MutationOptions) => request<T>(path, { method: "DELETE", headers: mutationHeaders(options) }),
};
