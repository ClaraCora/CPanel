import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Route } from "wouter";
import { ApiError, api } from "../api";
import { AdminNavigation } from "../components/AdminNavigation";
import { demoMachines, demoNodes, demoPlans, demoUsers, demoUserAccessIPs, demoSettings } from "../mock";
import { MachinesPage, UsersPage } from "./ResourcePages";
import { NodesPage } from "./NodesPage";
import { UserAccessIPsPage } from "./UserAccessIPsPage";
import { AuditPage } from "./AuditPage";
import { ToastProvider } from "../components/ui";
import { agentUpgradeState, canRequestUpgrade, upgradeStateLabel } from "../resourceModel";

beforeEach(() => {
  vi.restoreAllMocks();
  vi.stubGlobal("matchMedia", () => ({ matches: false }));
  vi.spyOn(api, "get").mockImplementation(async (path) => (path === "/machines" ? demoMachines : path === "/nodes" ? demoNodes : path === "/plans" ? demoPlans : path === "/users" ? demoUsers : path === "/settings/site" ? demoSettings.site : []) as never);
});

function machineEditor(id?: string) {
  history.replaceState({}, "", id ? `/machines/${id}/edit` : "/machines/new");
  return render(<AdminNavigation><Route path={id ? "/machines/:id/edit" : "/machines/new"} component={MachinesPage} /></AdminNavigation>);
}

describe("mutation recovery", () => {
  it("reuses a creation intent after a lost response, and starts another after the input changes", async () => {
    const post = vi.spyOn(api, "post").mockRejectedValue(new TypeError("Failed to fetch"));
    machineEditor();
    const form = await screen.findByRole("form");
    fireEvent.change(within(form).getByLabelText(/服务器名称/), { target: { value: "测试服务器" } });
    fireEvent.submit(form);
    await screen.findByText(/无法连接服务/);
    fireEvent.submit(form);
    await waitFor(() => expect(post).toHaveBeenCalledTimes(2));
    expect(post.mock.calls[0][2]?.idempotencyKey).toBe(post.mock.calls[1][2]?.idempotencyKey);
    fireEvent.change(within(form).getByLabelText(/服务器名称/), { target: { value: "另一个创建意图" } });
    fireEvent.submit(form);
    await waitFor(() => expect(post).toHaveBeenCalledTimes(3));
    expect(post.mock.calls[2][2]?.idempotencyKey).not.toBe(post.mock.calls[0][2]?.idempotencyKey);
  });

  it("sends the loaded config version and preserves input on conflict until discard is confirmed", async () => {
    const current = { ...demoMachines[0], config_version: 7 };
    vi.mocked(api.get).mockResolvedValue([current] as never);
    const patch = vi.spyOn(api, "patch").mockRejectedValue(new ApiError(409, { code: "CONFIG_VERSION_CONFLICT", message: "服务端配置已变化" }, "req-conflict"));
    machineEditor(current.id);
    const form = await screen.findByRole("form");
    fireEvent.change(within(form).getByLabelText(/服务器名称/), { target: { value: "尚未保存的名称" } });
    fireEvent.submit(form);
    await screen.findByText("服务端配置已变化");
    expect(patch).toHaveBeenCalledWith(`/machines/${current.id}`, expect.anything(), { configVersion: 7 });
    expect((screen.getByLabelText(/服务器名称/) as HTMLInputElement).value).toBe("尚未保存的名称");
    expect(screen.getByText(/请求 ID：/).textContent).toContain("req-conflict");
    fireEvent.click(screen.getByRole("button", { name: "放弃本页修改并重新加载" }));
    const dialog = screen.getByRole("alertdialog");
    expect((screen.getByLabelText(/服务器名称/) as HTMLInputElement).value).toBe("尚未保存的名称");
    vi.mocked(api.get).mockResolvedValue([{ ...current, name: "他人保存的新名称", config_version: 8 }] as never);
    fireEvent.click(within(dialog).getByRole("button", { name: "重新加载" }));
    await waitFor(() => expect((screen.getByLabelText(/服务器名称/) as HTMLInputElement).value).toBe("他人保存的新名称"));
  });
});

describe("upgrade and applied state", () => {
  it("offers a new upgrade after a previous successful terminal task", () => {
    const machine = { ...demoMachines[0], agent_upgrade_status: "succeeded" as const, agent_version: "v1", latest_agent_version: "v2", agent_upgrade_task_id: "old-task" };
    expect(agentUpgradeState(machine)).toBe("available");
    expect(canRequestUpgrade(machine)).toBe(true);
    expect(upgradeStateLabel(machine)).toBe("更新至 v2");
    const latest = { ...machine, agent_version: "v2" };
    const running = { ...machine, agent_upgrade_status: "acknowledged" as const };
    expect(agentUpgradeState(latest)).toBe("latest");
    expect(canRequestUpgrade(running)).toBe(false);
  });

  it("shows a published node's draft separately and refreshes Agent confirmation without leaving", async () => {
    history.replaceState({}, "", "/nodes");
    const node = { ...demoNodes[0], has_draft: true, current_revision: 2, applied_revision: 1 };
    vi.mocked(api.get).mockImplementation(async (path) => (path === "/nodes" ? [node] : demoMachines) as never);
    render(<AdminNavigation><NodesPage /></AdminNavigation>);
    await screen.findByText("有未发布修改");
    expect(screen.getByRole("button", { name: `发布 ${node.name}` })).toBeTruthy();
    expect(screen.getByText("等待 Agent 接收并应用")).toBeTruthy();
    node.applied_revision = 2;
    fireEvent.click(screen.getByRole("button", { name: "刷新" }));
    await waitFor(() => expect(screen.queryByText("等待 Agent 接收并应用")).toBeNull());
  });
});

describe("paged search and recovery", () => {
  it("trusts server IP search results and keeps navigation for an empty later page", async () => {
    history.replaceState({}, "", "/access-ips?q=深圳&page=2");
    const result = { items: [], total: 100, has_more: false };
    vi.mocked(api.get).mockResolvedValue(result as never);
    render(<UserAccessIPsPage />);
    await screen.findByText("没有匹配的访问 IP");
    expect(screen.getByRole("button", { name: "上一页" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "上一页" }));
    await waitFor(() => expect(api.get).toHaveBeenCalledWith(expect.stringContaining("page=1"), expect.anything()));
  });

  it("maps a displayed Chinese audit operation to the server action", async () => {
    history.replaceState({}, "", "/audit");
    vi.mocked(api.get).mockResolvedValue({ items: [], total: 0, has_more: false } as never);
    render(<AuditPage />);
    fireEvent.change(screen.getByRole("textbox", { name: "搜索审计日志" }), { target: { value: "发布节点" } });
    await waitFor(() => expect(api.get).toHaveBeenCalledWith(expect.stringContaining("q=node.publish"), expect.anything()));
    expect(new URLSearchParams(location.search).get("q")).toBe("发布节点");
  });
});
