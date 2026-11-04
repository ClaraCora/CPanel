import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "./api";
import { adminSessionExpiredEvent, authApi } from "./auth";
import { demoSession } from "./mock";

describe("admin API session handling", () => {
  afterEach(() => vi.restoreAllMocks());
  beforeEach(async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ data: demoSession }), { headers: { "Content-Type": "application/json" } }));
    await authApi.current();
  });

  it("announces an expired session when a protected endpoint returns 401", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({ data: null, error: { code: "UNAUTHENTICATED", message: "登录已过期" } }), {
      status: 401,
      headers: { "Content-Type": "application/json" },
    }));
    const expired = vi.fn();
    window.addEventListener(adminSessionExpiredEvent, expired, { once: true });

    await expect(api.get("/overview")).rejects.toMatchObject({ status: 401 });
    expect(expired).toHaveBeenCalledTimes(1);
  });

  it("reads a streamed 401 error and request ID before aborting other protected requests", async () => {
    let signal: AbortSignal | null | undefined;
    vi.mocked(fetch).mockImplementation(async (_url, init) => {
      signal = init?.signal;
      const encoder = new TextEncoder();
      const body = new ReadableStream({ start(controller) {
        controller.enqueue(encoder.encode('{"error":{"code":"ADMIN_SESSION_INVALID",'));
        window.setTimeout(() => { controller.enqueue(encoder.encode('"message":"会话过期"},"meta":{"request_id":"stream-request"}}')); controller.close(); }, 5);
      } });
      return new Response(body, { status: 401, headers: { "Content-Type": "application/json" } });
    });
    await expect(api.patch("/machines/mch", {})).rejects.toMatchObject({ status: 401, code: "ADMIN_SESSION_INVALID", requestId: "stream-request" });
    expect(signal?.aborted).toBe(false);
  });

  it("preserves metadata and sends idempotency and optimistic edit headers", async () => {
    vi.mocked(fetch).mockImplementation(async () => new Response(JSON.stringify({ data: [], meta: { request_id: "meta-request", config_version: 8 } }), { headers: { "Content-Type": "application/json" } }));
    expect(await api.getWithMeta("/settings/site")).toEqual({ data: [], meta: { request_id: "meta-request", config_version: 8 } });
    await api.post("/machines", { name: "one" }, { idempotencyKey: "stable-intent" });
    expect(new Headers(vi.mocked(fetch).mock.calls.at(-1)?.[1]?.headers).get("Idempotency-Key")).toBe("stable-intent");
    await api.patch("/machines/mch", { name: "two" }, { configVersion: 8 });
    expect(new Headers(vi.mocked(fetch).mock.calls.at(-1)?.[1]?.headers).get("If-Match")).toBe('"8"');
  });
});
