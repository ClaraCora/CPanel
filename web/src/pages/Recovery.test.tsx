import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
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
import { SubscriptionPage } from "./SettingsPage";
import PortalDashboardPage, { demoPortalDashboard } from "../PortalDashboardPage";
import { portalApi } from "../portalApi";

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
  it("requires a safe reload when the server omitted the edit version", async () => {
    const current = { ...demoMachines[0], config_version: undefined };
    vi.mocked(api.get).mockResolvedValue([current] as never);
    const patch = vi.spyOn(api, "patch");
    machineEditor(current.id);
    const form = await screen.findByRole("form");
    await screen.findByText("服务未返回配置版本，暂时无法安全保存。当前输入已保留。");
    fireEvent.change(within(form).getByLabelText(/服务器名称/), { target: { value: "需要保留的输入" } });
    fireEvent.submit(form);
    expect(patch).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "放弃本页修改并重新加载" }));
    const dialog = screen.getByRole("alertdialog");
    expect((screen.getByLabelText(/服务器名称/) as HTMLInputElement).value).toBe("需要保留的输入");
    vi.mocked(api.get).mockResolvedValue([{ ...current, name: "重新加载的名称", config_version: 2 }] as never);
    fireEvent.click(within(dialog).getByRole("button", { name: "重新加载" }));
    await waitFor(() => expect((screen.getByLabelText(/服务器名称/) as HTMLInputElement).value).toBe("重新加载的名称"));
    expect(screen.queryByText("服务未返回配置版本，暂时无法安全保存。当前输入已保留。")).toBeNull();
  });

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
  it("does not discard an IP record that the paged server matched", async () => {
    history.replaceState({}, "", "/access-ips?q=后端匹配的地区");
    vi.mocked(api.get).mockResolvedValue({ items: [demoUserAccessIPs[0]], total: 1, has_more: false } as never);
    render(<UserAccessIPsPage />);
    await screen.findByText(demoUserAccessIPs[0].user_name);
    expect(screen.queryByText("没有匹配的访问 IP")).toBeNull();
  });
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

  it.each(["发布节点", "节点", "编辑", "恢复", "出站"])("sends Chinese audit search %s unchanged for server-side matching", async (query) => {
    history.replaceState({}, "", "/audit");
    vi.mocked(api.get).mockResolvedValue({ items: [], total: 0, has_more: false } as never);
    render(<AuditPage />);
    fireEvent.change(screen.getByRole("textbox", { name: "搜索审计日志" }), { target: { value: query } });
    await waitFor(() => expect(vi.mocked(api.get).mock.calls.some(([path]) => new URL(path, location.origin).searchParams.get("q") === query)).toBe(true));
    expect(new URLSearchParams(location.search).get("q")).toBe(query);
    for (const label of ["恢复节点版本", "编辑套餐", "删除权限组", "编辑路由策略", "删除出站", "升级 Agent", "创建门户只读代入"]) {
      expect(screen.getByRole("option", { name: label })).toBeTruthy();
    }
  });
});

describe("settings and entitlement safeguards", () => {
  it("protects section settings with the envelope version and retains a conflicting draft", async () => {
    history.replaceState({}, "", "/subscription");
    vi.spyOn(api, "getWithMeta").mockResolvedValue({ data: demoSettings.subscription, meta: { request_id: "settings", config_version: 9 } } as never);
    const patch = vi.spyOn(api, "patch").mockRejectedValue(new ApiError(409, { code: "EDIT_VERSION_CONFLICT", message: "设置已变化" }));
    render(<SubscriptionPage />);
    const input = await screen.findByLabelText("订阅基础地址");
    fireEvent.change(input, { target: { value: "https://retained.example" } });
    fireEvent.click(screen.getByRole("button", { name: "保存设置" }));
    await screen.findByText("设置已变化");
    expect(patch.mock.calls[0][2]).toEqual({ configVersion: 9 });
    expect((screen.getByLabelText("订阅基础地址") as HTMLInputElement).value).toBe("https://retained.example");
  });

  it("requires an explicit expiry choice and excludes disabled plans from new assignments", async () => {
    const disabled = { ...demoPlans[1], id: "disabled-plan", status: "disabled", name: "停用套餐" };
    vi.mocked(api.get).mockImplementation(async (path) => (path === "/plans" ? [...demoPlans, disabled] : path === "/settings/site" ? demoSettings.site : []) as never);
    history.replaceState({}, "", "/users/new");
    render(<AdminNavigation><Route path="/users/new" component={UsersPage} /></AdminNavigation>);
    const form = await screen.findByRole("form");
    expect(within(form).queryByRole("option", { name: /停用套餐/ })).toBeNull();
    expect((within(form).getByLabelText("到期方式") as HTMLSelectElement).value).toBe("default");
    fireEvent.change(within(form).getByLabelText("到期方式"), { target: { value: "custom" } });
    expect(within(form).getByLabelText(/到期时间/)).toBeTruthy();
    fireEvent.change(within(form).getByLabelText("到期方式"), { target: { value: "permanent" } });
    expect(within(form).queryByLabelText(/到期时间/)).toBeNull();
    await screen.findByText("保存后长期有效。");
  });

  it("shows usage and the actual quota reason when there are no authorized portal nodes", async () => {
    vi.spyOn(portalApi, "dashboard").mockResolvedValue({ ...demoPortalDashboard, nodes: [], traffic_used_bytes: 1024, traffic_limit_bytes: 1024, access_unavailable_reason: "流量已用尽，重置或增加额度后恢复" });
    render(<PortalDashboardPage session={{ user: { id: "u", name: "user", email: null, role: "user", status: "active" }, csrf_token: "token", expires_at: "2030-01-01", read_only: false }} onLogout={() => undefined} />);
    await screen.findByText("流量已用尽，重置或增加额度后恢复");
    expect(screen.getByRole("heading", { name: "流量使用" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: /单节点链接/ })).toBeNull();
    expect((screen.getByRole("button", { name: "Clash / Mihomo" }) as HTMLButtonElement).disabled).toBe(true);
  });
});

describe("portal popup recovery", () => {
  function accountList() {
    history.replaceState({}, "", "/users");
    return render(<ToastProvider><AdminNavigation><UsersPage /></AdminNavigation></ToastProvider>);
  }

  it("opens a safe blank page before waiting for a slow grant", async () => {
    const replace = vi.fn();
    const popup = { opener: window, closed: false, document: { title: "", body: { textContent: "" } }, location: { replace }, close: vi.fn() };
    const open = vi.spyOn(window, "open").mockReturnValue(popup as unknown as Window);
    let resolve!: (value: { grant: string }) => void;
    const post = vi.spyOn(api, "post").mockImplementation(() => new Promise((done) => { resolve = done as typeof resolve; }));
    accountList();
    await screen.findByRole("table");
    fireEvent.click(screen.getByRole("button", { name: `${demoUsers[0].id} 更多操作` }));
    fireEvent.click(screen.getByRole("menuitem", { name: "进入订阅页面" }));
    expect(open).toHaveBeenCalledWith("about:blank", "_blank");
    expect(open.mock.invocationCallOrder[0]).toBeLessThan(post.mock.invocationCallOrder[0]);
    expect(popup.opener).toBeNull();
    expect(replace).not.toHaveBeenCalled();
    await act(async () => resolve({ grant: "one-time-grant" }));
    expect(replace).toHaveBeenCalledWith("/edu?grant=one-time-grant");
  });

  it("explains a blocked popup and does not issue an unused grant", async () => {
    vi.spyOn(window, "open").mockReturnValue(null);
    const post = vi.spyOn(api, "post");
    accountList();
    await screen.findByRole("table");
    fireEvent.click(screen.getByRole("button", { name: `${demoUsers[0].id} 更多操作` }));
    fireEvent.click(screen.getByRole("menuitem", { name: "进入订阅页面" }));
    await screen.findByText("浏览器阻止了新窗口，请允许本站弹出窗口后重试。");
    expect(post).not.toHaveBeenCalled();
  });
});
