import { useState } from "react";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ConfirmDialog, DetailDialog, RowMenu } from "./ui";
import { OverviewTraffic } from "./OverviewTraffic";
import { demoOverview } from "../mock";

describe("modal focus behavior", () => {
  it("traps focus inside a large detail dialog and returns it to the opener", () => {
    function Example() {
      const [open, setOpen] = useState(false);
      return <><button type="button" onClick={() => setOpen(true)}>打开</button><DetailDialog open={open} title="服务器详情" onClose={() => setOpen(false)}><input aria-label="服务器名称" readOnly /><button type="button">复制</button></DetailDialog></>;
    }

    render(<Example />);
    const opener = screen.getByRole("button", { name: "打开" });
    opener.focus();
    fireEvent.click(opener);
    const close = screen.getByRole("button", { name: "关闭对话框" });
    const save = screen.getByRole("button", { name: "复制" });
    expect(document.activeElement).toBe(close);
    save.focus();
    fireEvent.keyDown(document, { key: "Tab" });
    expect(document.activeElement).toBe(close);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(opener);
  });

  it("keeps a loading confirmation open when Escape is pressed", () => {
    const close = vi.fn();
    render(<ConfirmDialog open loading title="删除记录" description="操作不可撤销" onClose={close} onConfirm={vi.fn()} />);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(close).not.toHaveBeenCalled();
    expect(screen.getByRole("alertdialog")).toBeTruthy();
  });
});

describe("row menu focus", () => {
  it("keeps the selected item focused when the open menu is repositioned", async () => {
    render(<RowMenu label="更多操作"><button type="button">第一项</button><button type="button">第二项</button></RowMenu>);
    const trigger = screen.getByRole("button", { name: "更多操作" });
    vi.spyOn(trigger, "getBoundingClientRect").mockReturnValue(new DOMRect(20, 20, 32, 32));
    fireEvent.click(trigger);
    const first = screen.getByRole("menuitem", { name: "第一项" });
    const second = screen.getByRole("menuitem", { name: "第二项" });
    await waitFor(() => expect(document.activeElement).toBe(first));
    fireEvent.keyDown(first, { key: "ArrowDown" });
    expect(document.activeElement).toBe(second);
    fireEvent.scroll(window);
    await act(async () => {
      await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()));
    });
    await act(async () => {
      await new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()));
    });
    expect(document.activeElement).toBe(second);
  });
});

describe("overview traffic dates", () => {
  it("shows an empty chart for an invalid traffic day", () => {
    render(<OverviewTraffic data={{ ...demoOverview, traffic_day: "invalid-day" }} />);
    expect(screen.getByText("暂无近七日流量数据")).toBeTruthy();
  });

  it("uses the current-day totals when a date includes a time", () => {
    const day = "2026-11-25";
    render(<OverviewTraffic data={{ ...demoOverview, traffic_day: `${day}T00:00:00Z`, traffic_week: [{ day, upload_bytes: 10, download_bytes: 20 }], traffic_today_bytes: 100, traffic_today_upload_bytes: 40, traffic_today_download_bytes: 60 }} />);
    fireEvent.click(screen.getByRole("button", { name: `${day} 上传 10 B，下载 20 B` }));
    expect(screen.getByText("100 B")).toBeTruthy();
    expect(screen.getByText("今日上传")).toBeTruthy();
  });
});
