import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { api } from "../api";
import { demoOverview } from "../mock";
import { HistoryPage } from "./HistoryPage";
import { OverviewPage } from "./OverviewPage";

beforeEach(() => window.history.replaceState({}, "", "/?demo=1"));

describe("overview reporting", () => {
  it("keeps the period with its results through a failed switch and a retry", async () => {
    let failSwitch!: (reason: Error) => void;
    vi.spyOn(api, "get")
      .mockResolvedValueOnce({ ...demoOverview, node_traffic_ranking: [] })
      .mockImplementationOnce(() => new Promise((_, reject) => { failSwitch = reject; }))
      .mockResolvedValueOnce(demoOverview);
    render(<OverviewPage />);
    const attention = await screen.findByRole("region", { name: "需要关注" });
    const ranking = screen.getByRole("heading", { name: "今日流量排行" });
    expect(attention.compareDocumentPosition(ranking) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(screen.getByRole("link", { name: /台服务器离线/ }).getAttribute("href")).toBe("/machines?status=offline");

    fireEvent.click(screen.getByRole("button", { name: "昨日" }));
    expect(window.location.search).toContain("period=yesterday");
    expect(screen.getByText("正在加载昨日排行…")).toBeTruthy();
    expect(screen.getByText("今日还没有节点流量")).toBeTruthy();
    expect(screen.queryByRole("heading", { name: "昨日流量排行" })).toBeNull();
    await act(async () => failSwitch(new Error("请求超时")));
    expect(screen.getByRole("alert").textContent).toContain("昨日排行加载失败，当前显示今日排行");
    fireEvent.click(screen.getByRole("button", { name: "重新加载" }));
    await screen.findByRole("heading", { name: "昨日流量排行" });
    expect(screen.queryByText("今日还没有节点流量")).toBeNull();
  });
});

describe("history reporting", () => {
  it("loads the selected page independently of retention settings and resets pagination on a view switch", async () => {
    window.history.replaceState({}, "", "/history?view=devices&page=3&page_size=8");
    let retentionFailed = false;
    const get = vi.spyOn(api, "get").mockImplementation(async (path) => {
      if (path === "/settings/retention") {
        if (retentionFailed) return [{ key: "devices_days", value: "45" }];
        retentionFailed = true;
        throw new Error("读取失败");
      }
      if (path === "/history?view=devices&page=3&page_size=8") return { items: [], total: 16, has_more: false };
      if (path === "/history?view=machines&page=1&page_size=8") return { items: [{ resource_name: "测试服务器", resource_id: "m1", sampled_at: "2026-11-25T00:00:00Z", metrics: {} }], total: 1 };
      throw new Error("意外请求 " + path);
    });
    render(<HistoryPage />);
    await screen.findByText("这一页已没有设备连接记录");
    expect(screen.getByText("保留期读取失败")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "重新读取保留期" }));
    await screen.findByText("保留 45 天");
    fireEvent.click(screen.getByRole("button", { name: "服务器指标" }));
    await screen.findByText("测试服务器");
    expect(screen.getByText("仅保留最新值")).toBeTruthy();
    expect(get.mock.calls.map(([path]) => path)).not.toContain("/history");
  });
});
