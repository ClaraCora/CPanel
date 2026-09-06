import { demoNodes, demoResource, demoSession } from "./mock";
import { ApiError, demoMode, getCsrfToken, notifyAdminSessionExpired } from "./auth";
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
  if (segments[0] === "users" && tail[0] === "quick") {
    tail[0] = "ks";
    tail[1] = rolePaths[tail[1]] ?? tail[1];
  }
  if (segments[0] === "users" && tail[1] === "subscription") tail[1] = "dy";
	if (segments[0] === "users" && tail[1] === "portal") tail[1] = "edu";
  return `/${resource}${tail.length ? `/${tail.join("/")}` : ""}`;
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  if (demoMode) return demoRequest<T>(path, init);
  const headers = new Headers(init.headers);
  if (init.body) headers.set("Content-Type", "application/json");
  if (init.method && !["GET", "HEAD"].includes(init.method)) headers.set("X-CSRF-Token", getCsrfToken());
  const controller = new AbortController();
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
    if (response.status === 401) notifyAdminSessionExpired();
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
      if (!timedOut) throw reason;
      throw new ApiError(408, { code: "REQUEST_TIMEOUT", message: "请求超时，请检查网络后重试" });
    }
    throw reason;
  } finally {
    window.clearTimeout(timeout);
    init.signal?.removeEventListener("abort", abortFromCaller);
  }
}

async function demoRequest<T>(path: string, init: RequestInit): Promise<T> {
  await new Promise((resolve) => window.setTimeout(resolve, 120));
  const method = init.method ?? "GET";
  const cleanPath = path.split("?", 1)[0];
  if (cleanPath === "/sessions" && method === "POST") return demoSession as T;
  if (cleanPath === "/sessions/current" && method === "DELETE") return { logged_out: true } as T;
  if (cleanPath === "/account/profile" && method === "PATCH") {
    const input = JSON.parse(String(init.body));
    demoSession.admin = { ...demoSession.admin, ...input };
    return demoSession.admin as T;
  }
	if (cleanPath === "/account/password" && method === "PATCH") return { password_updated: true, other_sessions_revoked: 0 } as T;
	if (cleanPath === "/settings/tgbot/test" && method === "POST") return { sent: true } as T;
	if (cleanPath === "/user-access-ips/location" && method === "POST") {
		const input = JSON.parse(String(init.body));
		return { ip_address: input.ip_address, cached: false, location: { scope: "public", country_code: "CN", country: "中国", province: "广东省", city: "深圳", isp: "中国电信", resolved_at: new Date().toISOString() } } as T;
	}
	if (cleanPath.startsWith("/users/") && cleanPath.endsWith("/portal") && method === "POST") return { grant: "demo-portal-grant", expires_at: new Date(Date.now() + 90_000).toISOString() } as T;
	if (method === "GET") {
		const viewMatch = path.match(/^\/history\?view=(traffic|machines|nodes|devices)/);
		if (viewMatch) {
			const history = demoResource("/history") as { traffic?: unknown[]; machine_metrics?: unknown[]; node_metrics?: unknown[]; devices?: unknown[] };
			const source = viewMatch[1] === "traffic" ? history.traffic : viewMatch[1] === "machines" ? history.machine_metrics : viewMatch[1] === "nodes" ? history.node_metrics : history.devices;
			const items = Array.isArray(source) ? source : [];
			return { view: viewMatch[1], items, page: 1, page_size: items.length || 50, total: items.length, has_more: false } as T;
		}
		return demoResource(cleanPath) as T;
	}
  if (cleanPath === "/nodes" && method === "POST") {
    const input = JSON.parse(String(init.body));
    const item = { ...demoNodes[0], ...input, id: `nod_demo_${Date.now()}`, agent_id: Date.now() % 100000, machine_name: "演示服务器", status: "draft", current_revision: 0, applied_revision: 0 };
    demoNodes.unshift(item);
    return item as T;
  }
  if (cleanPath.startsWith("/nodes/") && method === "PATCH") {
    const id = cleanPath.split("/")[2];
    const index = demoNodes.findIndex((item) => item.id === id);
    const input = JSON.parse(String(init.body));
    demoNodes[index] = { ...demoNodes[index], ...input, status: "draft", updated_at: new Date().toISOString() };
    return demoNodes[index] as T;
  }
  if (cleanPath.endsWith("/publish") && method === "POST") {
    const id = cleanPath.split("/")[2];
    const item = demoNodes.find((node) => node.id === id)!;
    item.status = "published";
    item.current_revision += 1;
    return item as T;
  }
  const resource = cleanPath.split("/")[1];
  const input = init.body ? JSON.parse(String(init.body)) : {};
  return { ...input, id: `${resource}_${Date.now()}`, status: "active", created_at: new Date().toISOString(), updated_at: new Date().toISOString() } as T;
}

export const api = {
  get: <T>(path: string, signal?: AbortSignal) => request<T>(path, { signal }),
  post: <T>(path: string, body?: unknown) => request<T>(path, { method: "POST", body: body === undefined ? undefined : JSON.stringify(body) }),
  patch: <T>(path: string, body: unknown) => request<T>(path, { method: "PATCH", body: JSON.stringify(body) }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
};
