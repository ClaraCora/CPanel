import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Route, Switch } from "wouter";
import { ApiError, api } from "../api";
import { AdminNavigation } from "../components/AdminNavigation";
import { demoGroups, demoMachines, demoNodes, demoOutbounds, demoPlans, demoRoutes, demoSettings, demoUsers } from "../mock";
import { AccessGroupsPage, MachinesPage, OutboundsPage, PlansPage, RoutesPage, UsersPage } from "./ResourcePages";
import { NodeEditorPage } from "./NodeEditorPage";

const resources: Record<string, unknown> = { "/machines": demoMachines, "/nodes": demoNodes, "/access-groups": demoGroups, "/plans": demoPlans, "/users": demoUsers, "/route-policies": demoRoutes, "/outbounds": demoOutbounds };
beforeEach(() => {
  vi.stubGlobal("matchMedia", () => ({ matches: false }));
  vi.spyOn(api, "get").mockImplementation(async (path) => {
    if (path.startsWith("/settings/")) return (demoSettings[path.split("/")[2]] ?? []) as never;
    if (path.startsWith("/nodes/")) return demoNodes.find((node) => node.id === path.split("/")[2]) as never;
    return resources[path] as never;
  });
});

function open(Component: typeof MachinesPage, base: string, suffix: string) {
  history.replaceState({}, "", base + suffix);
  return render(<AdminNavigation><Switch><Route path={`${base}/new`} component={Component} /><Route path={`${base}/:id/edit`} component={Component} /><Route path={base}><h1>返回的列表</h1></Route></Switch></AdminNavigation>);
}

describe("standalone resource editors", () => {
  it.each([
    [MachinesPage, "/machines", demoMachines[0].id, "编辑服务器", "服务器名称", demoMachines[0].name],
    [AccessGroupsPage, "/access-groups", demoGroups[0].id, "编辑权限组", "权限组名称", demoGroups[0].name],
    [PlansPage, "/plans", demoPlans[0].id, "编辑套餐", "套餐名称", demoPlans[0].name],
    [UsersPage, "/users", demoUsers[0].id, "编辑订阅账号", "账号名称", demoUsers[0].name],
    [RoutesPage, "/routes", demoRoutes[0].id, "编辑路由策略", "策略名称", demoRoutes[0].name],
    [OutboundsPage, "/outbounds", demoOutbounds[0].id, "编辑出站", "出站名称", demoOutbounds[0].name],
  ] as const)("loads resource directly from the existing list API", async (Component, base, id, title, label, value) => {
    open(Component, base, `/${id}/edit`);
    const form = await screen.findByRole("form", { name: title });
    expect((within(form).getByLabelText(new RegExp(label)) as HTMLInputElement).value).toBe(value);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(api.get).not.toHaveBeenCalledWith(`${base}/${id}`, expect.anything());
  });

  it("never turns a missing record into a create form", async () => {
    open(MachinesPage, "/machines", "/missing/edit");
    await screen.findByText("服务器不存在");
    expect(screen.queryByRole("form")).toBeNull();
  });

  it("preserves input and field errors after a failed save", async () => {
    vi.spyOn(api, "patch").mockRejectedValue(new ApiError(422, { code: "VALIDATION_ERROR", message: "名称重复", fields: { name: "请换一个名称" } }));
    open(MachinesPage, "/machines", `/${demoMachines[0].id}/edit`);
    const form = await screen.findByRole("form");
    fireEvent.change(within(form).getByLabelText(/服务器名称/), { target: { value: "新的服务器名称" } });
    fireEvent.submit(form);
    await screen.findByText("名称重复");
    expect((screen.getByLabelText(/服务器名称/) as HTMLInputElement).value).toBe("新的服务器名称");
    expect(screen.getByLabelText(/服务器名称/).getAttribute("aria-invalid")).toBe("true");
  });

  it("converts plan GB with 1024 and restores the encoded list query", async () => {
    vi.spyOn(api, "patch").mockResolvedValue(demoPlans[0]);
    open(PlansPage, "/plans", `/${demoPlans[0].id}/edit?returnTo=${encodeURIComponent("/plans?q=标准&status=active")}`);
    const form = await screen.findByRole("form");
    fireEvent.change(within(form).getByLabelText(/套餐流量/), { target: { value: "12" } });
    fireEvent.submit(form);
    await screen.findByText("返回的列表");
    expect(api.patch).toHaveBeenCalledWith(`/plans/${demoPlans[0].id}`, expect.objectContaining({ traffic_limit_bytes: 12 * 1024 ** 3 }));
    expect(new URLSearchParams(location.search).get("q")).toBe("标准");
  });

  it("creates a user without requiring the edit-only UUID", async () => {
    vi.spyOn(api, "post").mockResolvedValue(demoUsers[0]);
    open(UsersPage, "/users", "/new");
    const form = await screen.findByRole("form");
    fireEvent.change(within(form).getByLabelText(/账号名称/), { target: { value: "新用户" } });
    fireEvent.submit(form);
    await screen.findByText("返回的列表");
    expect(api.post).toHaveBeenCalledWith("/users", expect.objectContaining({ name: "新用户", role: "user" }));
  });

  it("keeps a newly saved node after publish failure and retries only publication", async () => {
    const saved = { ...demoNodes[1], id: "created-node", name: "测试节点" };
    vi.mocked(api.get).mockImplementation(async (path) => path === "/nodes/created-node" ? saved as never : (path === "/machines" ? demoMachines : path === "/route-policies" ? demoRoutes : []) as never);
    const post = vi.spyOn(api, "post").mockImplementation(async (path) => {
      if (path === "/nodes") return saved as never;
      throw new Error("Agent 暂不可用");
    });
    const patch = vi.spyOn(api, "patch");
    open(NodeEditorPage, "/nodes", "/new");
    const form = await screen.findByRole("form", { name: "添加节点" });
    fireEvent.change(within(form).getByLabelText(/节点名称/), { target: { value: "测试节点" } });
    fireEvent.click(within(form).getByRole("button", { name: "保存并发布" }));
    await screen.findByRole("form", { name: "编辑节点" });
    await screen.findByText(/配置已保存，发布失败/);
    fireEvent.click(screen.getByRole("button", { name: "重试发布" }));
    await waitFor(() => expect(post.mock.calls.filter(([path]) => path.endsWith("/publish"))).toHaveLength(2));
    expect(post.mock.calls.filter(([path]) => path === "/nodes")).toHaveLength(1);
    expect(patch).not.toHaveBeenCalled();
    expect(location.pathname).toBe("/nodes/created-node/edit");
  });
});
