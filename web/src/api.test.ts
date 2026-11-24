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

  it("keeps the streamed 401 error and request ID readable after expiring the session", async () => {
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

  it("expires the session even if the 401 JSON body is incomplete", async () => {
    vi.mocked(fetch).mockResolvedValue(new Response("{", { status: 401, headers: { "Content-Type": "application/json" } }));
    const expired = vi.fn();
    window.addEventListener(adminSessionExpiredEvent, expired, { once: true });
    await expect(api.get("/overview")).rejects.toThrow();
    expect(expired).toHaveBeenCalledTimes(1);
    await expect(api.get("/overview")).rejects.toMatchObject({ status: 401 });
  });

  it("preserves metadata and sends idempotency and optimistic edit headers", async () => {
    vi.mocked(fetch).mockImplementation(async () => new Response(JSON.stringify({ data: [], meta: { request_id: "meta-request", config_version: 8 } }), { headers: { "Content-Type": "application/json" } }));
    expect(await api.getWithMeta("/settings/site")).toEqual({ data: [], meta: { request_id: "meta-request", config_version: 8 } });
    await api.post("/machines", { name: "one" }, { idempotencyKey: "stable-intent" });
    expect(new Headers(vi.mocked(fetch).mock.calls.at(-1)?.[1]?.headers).get("Idempotency-Key")).toBe("stable-intent");
    await api.patch("/machines/mch", { name: "two" }, { configVersion: 8 });
    expect(new Headers(vi.mocked(fetch).mock.calls.at(-1)?.[1]?.headers).get("If-Match")).toBe('"8"');
    await api.delete("/nodes/node", { configVersion: 12 });
    expect(new Headers(vi.mocked(fetch).mock.calls.at(-1)?.[1]?.headers).get("If-Match")).toBe('"12"');
  });
  it("uses authenticated fleet routes for create, restore, progress and single-machine retries", async () => {
    vi.mocked(fetch).mockImplementation(async () => new Response(JSON.stringify({ data: {} }), { headers: { "Content-Type": "application/json" } }));
    await api.post("/agent-upgrade-batches", { target_version: "v2.1" }, { idempotencyKey: "create-fleet" });
    await api.get("/agent-upgrade-batches/latest");
    await api.get("/agent-upgrade-batches/batch-id");
    await api.post("/agent-upgrade-batches/batch-id/machines/machine-id/retry", {}, { idempotencyKey: "retry-machine" });
    const calls = vi.mocked(fetch).mock.calls.slice(-4);
    expect(calls.map(([url]) => url)).toEqual(["/ca/ht/sjpc", "/ca/ht/sjpc/zx", "/ca/ht/sjpc/batch-id", "/ca/ht/sjpc/batch-id/fwq/machine-id/cs"]);
    expect(new Headers(calls[0][1]?.headers).get("X-CSRF-Token")).toBeTruthy();
    expect(new Headers(calls[3][1]?.headers).get("Idempotency-Key")).toBe("retry-machine");
  });

});
