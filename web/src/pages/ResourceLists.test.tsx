import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderResource } from "../test/renderResource";
import { api } from "../api";
import { demoGroups, demoMachines, demoNodes, demoPlans, demoUsers } from "../mock";
import type { User } from "../types";
import { NodesPage } from "./NodesPage";
import { MachinesPage, PlansPage, UsersPage } from "./ResourcePages";

vi.mock("../api", async (original) => ({ ...await original<typeof import("../api")>(), demoMode: true }));

const accounts: User[] = [
  { ...demoUsers[0], id: "user-a", name: "测试成员甲" },
  { ...demoUsers[0], id: "user-b", name: "测试成员乙", status: "paused" },
  { ...demoUsers[0], id: "user-c", name: "测试成员丙", plan_name: "朋友共享" },
  { ...demoUsers[0], id: "friend-a", name: "测试朋友", role: "friend" },
  { ...demoUsers[0], id: "admin-a", name: "测试管理员", role: "admin" },
];

beforeEach(() => {
  window.history.replaceState({}, "", "/?demo=1");
  const resources: Record<string, unknown> = {
    "/machines": demoMachines, "/nodes": demoNodes, "/plans": demoPlans, "/users": accounts, "/access-groups": demoGroups,
  };
  vi.spyOn(api, "get").mockImplementation(async (path) => resources[path] as never);
});

describe("clear resource list workflows", () => {
  it("searches account values and partial identifiers without matching JSON field names", async () => {
    const accountsForSearch = [
      { ...demoUsers[0], name: "MC", email: null, portal_login: "member", uuid: demoUsers[0].uuid.toUpperCase() },
      { ...demoUsers[1], name: "朋友", email: "friend@example.test", portal_login: "buddy", id: "usr_123xyz456", notes: "续费提醒" },
    ];
    vi.mocked(api.get).mockImplementation(async (path) => (path === "/users" ? accountsForSearch : demoPlans) as never);
    renderResource(UsersPage, "/users");
    expect(screen.getByRole("button", { name: "全部" })).toBeTruthy();
    expect(screen.queryByText("0 条记录")).toBeNull();
    const table = await screen.findByRole("table");
    const input = screen.getByRole("textbox", { name: "搜索订阅账号" });
    for (const [query, expected] of [
      ["mc", "MC"], ["buddy", "朋友"], ["@example.test", "朋友"],
      ["123XYZ", "朋友"], [demoUsers[0].uuid.slice(7, 15), "MC"],
      ["标准套餐", "MC"], ["续费", "朋友"], ["2026-12-31", "MC"],
    ]) {
      fireEvent.change(input, { target: { value: query } });
      expect(within(table).getAllByRole("row")).toHaveLength(2);
      expect(within(table).getByRole("button", { name: `查看并编辑 ${expected}` })).toBeTruthy();
    }
    fireEvent.change(input, { target: { value: "subscription_available" } });
    expect(screen.getByText("没有符合搜索条件的记录")).toBeTruthy();
  });

  it("opens each destination from the expanded plan table independently", async () => {
    renderResource(PlansPage, "/plans");
    fireEvent.click(await screen.findByText("完整表格与操作"));
    const table = screen.getByRole("table", { name: "套餐列表" });
    const group = within(table).getByRole("link", { name: "标准线路" });
    const plan = within(table).getByRole("link", { name: "查看并编辑 标准套餐" });
    expect(new URL((plan as HTMLAnchorElement).href).pathname).toBe("/plans/pln_standard/edit");
    expect(new URL((group as HTMLAnchorElement).href).pathname).toBe("/access-groups/grp_standard/edit");
    expect(group.parentElement?.closest("a, button")).toBeNull();
  });

  it("shows only the latest apply failure and explicit error statuses under 异常", async () => {
    const nodes = demoNodes.map((node, index) => index === 0 || index === 2
      ? { ...node, applied_revision: node.current_revision - 1, apply_status: "failed", attempted_revision: node.current_revision - (index === 2 ? 1 : 0) }
      : node);
    vi.mocked(api.get).mockImplementation(async (path) => (path === "/nodes" ? nodes : demoMachines) as never);
    render(<NodesPage />);
    const table = await screen.findByRole("table");
    fireEvent.click(screen.getByRole("button", { name: /异常\s*2/ }));
    expect(within(table).getAllByRole("row")).toHaveLength(3);
    expect(within(table).getByText(demoNodes[0].name)).toBeTruthy();
    expect(within(table).getByText(demoNodes[3].name)).toBeTruthy();
    expect(within(table).queryByText(demoNodes[2].name)).toBeNull();
  });

  it("combines role, status, plan, and search, then clears all filters", async () => {
    renderResource(UsersPage, "/users");
    const table = await screen.findByRole("table", { name: "订阅账号列表" });
    fireEvent.click(screen.getByRole("button", { name: /用户\s*3/ }));
    fireEvent.change(screen.getByLabelText("按状态筛选订阅账号"), { target: { value: "active" } });
    fireEvent.change(screen.getByLabelText("按套餐筛选订阅账号"), { target: { value: "标准套餐" } });
    fireEvent.change(screen.getByRole("textbox", { name: "搜索订阅账号" }), { target: { value: "测试成员" } });
    expect(within(table).getAllByRole("row")).toHaveLength(2);
    expect(within(table).getByText("测试成员甲")).toBeTruthy();

    fireEvent.change(screen.getByRole("textbox", { name: "搜索订阅账号" }), { target: { value: "不存在的账号" } });
    fireEvent.click(screen.getByRole("button", { name: "清除筛选" }));
    expect(screen.getAllByRole("row")).toHaveLength(accounts.length + 1);
    expect((screen.getByLabelText("按套餐筛选订阅账号") as HTMLSelectElement).value).toBe("");
  });

  it("preserves admin protection in the menu and editor", async () => {
    renderResource(UsersPage, "/users");
    await screen.findByRole("table");
    fireEvent.click(screen.getByRole("button", { name: "测试管理员 更多操作" }));
    const menu = screen.getByRole("menu");
    expect(within(menu).queryByRole("menuitem", { name: "暂停" })).toBeNull();
    expect(within(menu).queryByRole("menuitem", { name: "删除" })).toBeNull();
    expect(within(menu).getByRole("menuitem", { name: "进入订阅页面" })).toBeTruthy();
    fireEvent.click(within(menu).getByRole("menuitem", { name: "编辑" }));
    const editor = await screen.findByRole("form", { name: "编辑订阅账号" });
    expect((within(editor).getByLabelText("账号分类") as HTMLSelectElement).disabled).toBe(true);
    expect((within(editor).getByLabelText(/^门户登录账号/) as HTMLInputElement).disabled).toBe(true);
    expect((within(editor).getByLabelText(/^设置或重置门户密码/) as HTMLInputElement).disabled).toBe(true);
  });

  it("combines machine category, region, and search without mutating data", async () => {
    renderResource(MachinesPage, "/machines");
    await screen.findByText("香港边缘 01");
    fireEvent.click(screen.getByRole("button", { name: "表格视图" }));
    fireEvent.click(screen.getByRole("button", { name: /在线\s*2/ }));
    fireEvent.change(screen.getByLabelText("按区域筛选服务器"), { target: { value: "香港" } });
    fireEvent.change(screen.getByRole("textbox", { name: "搜索服务器" }), { target: { value: "hk01" } });
    expect(screen.getAllByRole("row")).toHaveLength(2);
    expect(screen.getByText("香港边缘 01")).toBeTruthy();
  });

  it("keeps machine and search in the URL when choosing a node category", async () => {
    render(<NodesPage />);
    await screen.findByRole("table");
    fireEvent.change(screen.getByRole("textbox", { name: "搜索节点" }), { target: { value: "hysteria2" } });
    fireEvent.change(screen.getByLabelText("按服务器筛选"), { target: { value: "mch_hk01" } });
    fireEvent.click(screen.getByRole("button", { name: /待发布\s*1/ }));
    expect(screen.getAllByRole("row")).toHaveLength(2);
    const query = new URLSearchParams(window.location.search);
    expect(Object.fromEntries(query)).toEqual({ demo: "1", q: "hysteria2", machine: "mch_hk01", status: "draft" });
    const copy = screen.getByRole("button", { name: "香港 Hysteria2 更多操作" });
    fireEvent.click(copy);
    expect(new URL(screen.getByRole("menuitem", { name: "复制节点" }).getAttribute("href")!, window.location.origin).searchParams.get("copy")).toBe("nod_1046");
  });
});
