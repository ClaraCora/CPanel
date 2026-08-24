import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { api } from "./api";
import { useResource } from "./hooks";

describe("useResource", () => {
  it("prevents an older response from replacing a newer reload", async () => {
    let resolveFirst: ((value: string) => void) | undefined;
    vi.spyOn(api, "get")
      .mockImplementationOnce((_path, signal) => new Promise<string>((resolve, reject) => {
        resolveFirst = resolve;
        signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")), { once: true });
      }))
      .mockResolvedValueOnce("new");

    const { result } = renderHook(() => useResource("/resource", "initial"));
    await act(async () => { await result.current.reload(); });
    resolveFirst?.("old");
    await waitFor(() => expect(result.current.data).toBe("new"));
  });

  it("keeps the last data and marks it stale after a background refresh fails", async () => {
    vi.spyOn(api, "get").mockResolvedValueOnce("current").mockRejectedValueOnce(new Error("network unavailable"));
    const { result } = renderHook(() => useResource("/resource", "initial"));
    await waitFor(() => expect(result.current.data).toBe("current"));
    await act(async () => { await result.current.reload(true); });
    expect(result.current.data).toBe("current");
    expect(result.current.stale).toBe(true);
    expect(result.current.refreshError).toBe("network unavailable");
    expect(result.current.lastSuccessAt).not.toBeNull();
  });
});
