import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, api } from "../api";
import { AdminNavigation } from "../components/AdminNavigation";
import { ToastProvider } from "../components/ui";
import { demoMachines, demoNodes } from "../mock";
import type { Node } from "../types";
import { NodesPage } from "./NodesPage";

let current: Node;
beforeEach(() => {
  vi.restoreAllMocks();
  current = { ...demoNodes[0], config_version: 4, status: "draft", has_draft: true };
  history.replaceState({}, "", "/nodes");
  vi.spyOn(api, "get").mockImplementation(async (path) => (path === "/nodes" ? [{ ...current }] : demoMachines) as never);
});

function open() {
  return render(<ToastProvider><AdminNavigation><NodesPage /></AdminNavigation></ToastProvider>);
}

function menu() { fireEvent.click(screen.getByRole("button", { name: `${current.name} 更多操作` })); }

describe("node list mutation safeguards", () => {
  it("disables existing-node writes without a valid config version while keeping copy navigation", async () => {
    current.config_version = undefined;
    const post = vi.spyOn(api, "post"), patch = vi.spyOn(api, "patch"), remove = vi.spyOn(api, "delete");
    open(); await screen.findByRole("table");
    expect(screen.getByText("缺少配置版本，请刷新后再操作")).toBeTruthy();
    expect((screen.getByRole("button", { name: `发布 ${current.name}` }) as HTMLButtonElement).disabled).toBe(true);
    menu();
    expect((screen.getByRole("menuitem", { name: "停用节点" }) as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByRole("menuitem", { name: "删除节点" }) as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByRole("menuitem", { name: "复制节点" }).getAttribute("href")).toContain(`copy=${current.id}`);
    expect(post).not.toHaveBeenCalled(); expect(patch).not.toHaveBeenCalled(); expect(remove).not.toHaveBeenCalled();
  });

  it("sends the confirmed version for status changes and deletion", async () => {
    const patch = vi.spyOn(api, "patch").mockImplementation(async () => { current = { ...current, status: "disabled", config_version: 5 }; return current as never; });
    const remove = vi.spyOn(api, "delete").mockResolvedValue({} as never);
    open(); await screen.findByRole("table");
    menu(); fireEvent.click(screen.getByRole("menuitem", { name: "停用节点" }));
    fireEvent.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "确认停用" }));
    await waitFor(() => expect(patch).toHaveBeenCalledWith(`/nodes/${current.id}`, { status: "disabled" }, { configVersion: 4 }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    await screen.findByRole("table");
    menu(); fireEvent.click(screen.getByRole("menuitem", { name: "删除节点" }));
    fireEvent.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: "删除节点" }));
    await waitFor(() => expect(remove).toHaveBeenCalledWith(`/nodes/${current.id}`, { configVersion: 5 }));
  });

  it("confirms an uncertain publication with the original key and version after refreshed data advances", async () => {
    const post = vi.spyOn(api, "post").mockRejectedValueOnce(new TypeError("Lost response")).mockResolvedValueOnce({} as never);
    open(); await screen.findByRole("table");
    fireEvent.click(screen.getByRole("button", { name: `发布 ${current.name}` }));
    await screen.findByText("上次发布未完成，可重试确认");
    current = { ...current, status: "published", config_version: 5, current_revision: current.current_revision + 1, has_draft: false };
    fireEvent.click(screen.getByRole("button", { name: "刷新" }));
    await screen.findByRole("table");
    fireEvent.click(screen.getByRole("button", { name: `重试确认发布 ${current.name}` }));
    await waitFor(() => expect(post).toHaveBeenCalledTimes(2));
    expect(post.mock.calls[0][2]?.idempotencyKey).toBeTruthy();
    expect(post.mock.calls[1][2]).toEqual(post.mock.calls[0][2]);
    expect(post.mock.calls[1][2]?.configVersion).toBe(4);
    await waitFor(() => expect(screen.queryByText("上次发布未完成，可重试确认")).toBeNull());
  });

  it("requires explicit refresh after an edit conflict before starting another publication intent", async () => {
    const post = vi.spyOn(api, "post").mockRejectedValueOnce(new ApiError(409, { code: "EDIT_VERSION_CONFLICT", message: "配置版本冲突" })).mockResolvedValueOnce({} as never);
    open(); await screen.findByRole("table");
    fireEvent.click(screen.getByRole("button", { name: `发布 ${current.name}` }));
    await screen.findByText("节点配置已变化，请刷新列表并检查最新状态后再操作。");
    expect((screen.getByRole("button", { name: `发布 ${current.name}` }) as HTMLButtonElement).disabled).toBe(true);
    current = { ...current, config_version: 9 };
    fireEvent.click(screen.getByRole("button", { name: "刷新并检查最新状态" }));
    await screen.findByRole("table");
    fireEvent.click(screen.getByRole("button", { name: `发布 ${current.name}` }));
    await waitFor(() => expect(post).toHaveBeenCalledTimes(2));
    expect(post.mock.calls[1][2]?.idempotencyKey).not.toBe(post.mock.calls[0][2]?.idempotencyKey);
    expect(post.mock.calls[1][2]?.configVersion).toBe(9);
  });
});
