import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "./api";
import { adminSessionExpiredEvent } from "./auth";

describe("admin API session handling", () => {
  afterEach(() => vi.restoreAllMocks());

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
});
