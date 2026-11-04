import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderResource } from "../test/renderResource";
import { api } from "../api";
import { portalApi } from "../portalApi";
import { demoOutbounds, demoRoutes, demoSettings } from "../mock";
import PortalDashboardPage, { demoPortalDashboard } from "../PortalDashboardPage";
import { OutboundsPage, RoutesPage } from "./ResourcePages";
import { SettingsPage, SubscriptionPage } from "./SettingsPage";

beforeEach(() => {
  window.history.replaceState({}, "", "/?demo=1");
  vi.stubGlobal("matchMedia", () => ({ matches: false }));
  vi.spyOn(api, "getWithMeta").mockImplementation(async (path) => ({ data: demoSettings[path.split("/")[2]] ?? [], meta: { request_id: "test", config_version: 1 } }) as never);
  vi.spyOn(api, "get").mockImplementation(async (path) => {
    if (path === "/outbounds") return demoOutbounds as never;
    if (path === "/route-policies") return demoRoutes as never;
    if (path.startsWith("/settings/subscription/access-log")) return { items: [], total: 0, page: 1, page_size: 50, has_more: false } as never;
    if (path.startsWith("/settings/")) return (demoSettings[path.split("/")[2]] ?? []) as never;
    return [] as never;
  });
});

describe("Clear A operations safeguards", () => {
  it("opens each route from its own rules without contaminating another record", async () => {
    renderResource(RoutesPage, "/routes");
    await screen.findByRole("table");
    fireEvent.click(screen.getByRole("button", { name: `编辑 ${demoRoutes[0].name}` }));
    let dialog = await screen.findByRole("form", { name: "编辑路由策略" });
    expect((within(dialog).getByLabelText(/策略名称/) as HTMLInputElement).value).toBe(demoRoutes[0].name);
    expect(within(dialog).getByDisplayValue("流媒体直连")).toBeTruthy();
    fireEvent.click(within(dialog).getByRole("button", { name: "取消" }));
    await screen.findByRole("table");
    fireEvent.click(screen.getByRole("button", { name: `编辑 ${demoRoutes[1].name}` }));
    dialog = await screen.findByRole("form", { name: "编辑路由策略" });
    expect(within(dialog).getByDisplayValue("内网直连")).toBeTruthy();
    expect(within(dialog).queryByDisplayValue("流媒体直连")).toBeNull();
  });

  it("opens the selected outbound's real server and upstream settings", async () => {
    const records = [0, 1].map((index) => ({ ...demoOutbounds[0], id: `out-${index}`, tag: `tag-${index}`, protocol: "socks", proxy_tag: index ? "tag-0" : "", name: `出站${index}`, settings: { servers: [{ address: `server-${index}.example.com`, port: 1080 + index }] } }));
    vi.mocked(api.get).mockImplementation(async () => records as never);
    renderResource(OutboundsPage, "/outbounds");
    await screen.findByRole("table");
    fireEvent.click(screen.getByRole("button", { name: `编辑 ${records[1].name}` }));
    const dialog = await screen.findByRole("form", { name: "编辑出站" });
    expect((within(dialog).getByLabelText(/服务器地址/) as HTMLInputElement).value).toBe("server-1.example.com");
    expect((within(dialog).getByLabelText(/服务器端口/) as HTMLInputElement).value).toBe("1081");
    expect((within(dialog).getByLabelText("上游出站") as HTMLSelectElement).value).toBe("tag-0");
  });

  it("retains unsaved subscription settings when collapsed and expanded", async () => {
    render(<SubscriptionPage />);
    const input = await screen.findByLabelText("订阅基础地址");
    fireEvent.change(input, { target: { value: "https://draft.example.com" } });
    fireEvent.click(screen.getByRole("button", { name: "收起设置" }));
    expect(screen.queryByRole("textbox", { name: "订阅基础地址" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "展开设置" }));
    expect((screen.getByLabelText("订阅基础地址") as HTMLInputElement).value).toBe("https://draft.example.com");
    expect((screen.getByRole("button", { name: "保存设置" }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("retains a settings draft across category switches", async () => {
    render(<SettingsPage admin={{ id: "adm", name: "管理", email: "admin@example.com", status: "active" }} onAdminChange={() => undefined} />);
    fireEvent.click(screen.getByRole("button", { name: "站点" }));
    const input = await screen.findByLabelText("平台名称");
    fireEvent.change(input, { target: { value: "未保存草稿" } });
    fireEvent.click(screen.getByRole("button", { name: "安全" }));
    fireEvent.click(screen.getByRole("button", { name: "站点" }));
    expect((screen.getByLabelText("平台名称") as HTMLInputElement).value).toBe("未保存草稿");
  });

  it("keeps delegated portal access read only while retaining copy actions", async () => {
    vi.spyOn(portalApi, "dashboard").mockResolvedValue(demoPortalDashboard);
    render(<PortalDashboardPage session={{ user: { id: "u", name: "user", email: null, role: "user", status: "active" }, csrf_token: "test", expires_at: "2030-01-01", read_only: true }} onLogout={() => undefined} />);
    await screen.findByRole("heading", { name: "账户中心" });
    expect(screen.getByRole("button", { name: "复制" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: /Clash \/ Mihomo/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /重置订阅/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /更新密码/ })).toBeNull();
    expect(screen.queryByLabelText("当前密码")).toBeNull();
    expect(screen.getAllByRole("button", { name: /复制链接/ })).toHaveLength(demoPortalDashboard.nodes.length);
  });
  it("restores focus to the reset action after closing its password dialog", async () => {
    vi.spyOn(portalApi, "dashboard").mockResolvedValue(demoPortalDashboard);
    render(<PortalDashboardPage session={{ user: { id: "u", name: "user", email: null, role: "user", status: "active" }, csrf_token: "test", expires_at: "2030-01-01", read_only: false }} onLogout={() => undefined} />);
    const trigger = await screen.findByRole("button", { name: "重置订阅地址" });
    trigger.focus();
    fireEvent.click(trigger);
    const dialog = screen.getByRole("dialog", { name: "重置订阅地址" });
    const input = within(dialog).getByLabelText("当前密码");
    expect(document.activeElement).toBe(input);
    fireEvent.keyDown(input, { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });

});
