import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { Route, Switch } from "wouter";
import { api } from "../api";
import { AdminNavigation } from "../components/AdminNavigation";
import { demoNodes } from "../mock";
import { redactRevision, revisionDiff } from "../nodeRevisionDiff";
import { NodeHistoryPage } from "./NodeHistoryPage";

const revisions = [
  { revision: 2, created_at: "2026-11-04T07:00:00Z", restorable: true, config: {}, snapshot: { node: { name: "节点", server_port: 2080 }, spec: { settings: { password: "new-secret", transport: "tcp" } } } },
  { revision: 1, created_at: "2026-11-03T07:00:00Z", restorable: false, config: { password: "old-secret", transport: "ws" } },
];

beforeEach(() => {
  history.replaceState({}, "", "/nodes/nod_history/history?returnTo=" + encodeURIComponent("/nodes?q=香港&status=published"));
  vi.spyOn(api, "get").mockImplementation(async (path) => (path.endsWith("/revisions") ? revisions : { ...demoNodes[0], id: "nod_history", has_draft: true, current_revision: 2, config_version: 4 }) as never);
});

function open() {
  return render(<AdminNavigation><Switch><Route path="/nodes/:id/history" component={NodeHistoryPage} /><Route path="/nodes/:id/edit"><h1>节点编辑页面</h1></Route></Switch></AdminNavigation>);
}

it("shows meaningful configuration changes without exposing secrets", async () => {
  open();
  await screen.findByRole("table", { name: "配置差异" });
  expect(screen.getByText("config.transport")).toBeTruthy();
  expect(screen.getAllByText("[已隐藏]").length).toBeGreaterThan(0);
  expect(document.body.textContent).not.toContain("old-secret");
  expect(document.body.textContent).not.toContain("new-secret");
  fireEvent.click(screen.getByRole("button", { name: /版本 1/ }));
  expect(screen.getByRole("button", { name: "恢复为草稿" }).hasAttribute("disabled")).toBe(true);
  expect(screen.getByText(/缺少完整依赖快照/)).toBeTruthy();
});

it("requires confirmation, preserves failed history, and restores only a draft", async () => {
  const post = vi.spyOn(api, "post").mockRejectedValueOnce(new Error("恢复请求失败")).mockResolvedValueOnce({});
  open(); await screen.findByRole("table");
  fireEvent.click(screen.getByRole("button", { name: "恢复为草稿" }));
  expect(screen.getByRole("alertdialog").textContent).toContain("当前未发布草稿将被替换");
  expect(post).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "恢复并编辑" }));
  await screen.findByText("恢复请求失败");
  expect(location.pathname).toBe("/nodes/nod_history/history");
  fireEvent.click(screen.getByRole("button", { name: "恢复为草稿" }));
  fireEvent.click(screen.getByRole("button", { name: "恢复并编辑" }));
  await screen.findByRole("heading", { name: "节点编辑页面" });
  expect(post).toHaveBeenCalledTimes(2);
  expect(post.mock.calls.every(([path]) => path === "/nodes/nod_history/restore")).toBe(true);
  const restoredReturn = new URL(new URLSearchParams(location.search).get("returnTo")!, location.origin);
  expect(restoredReturn.pathname).toBe("/nodes");
  expect(restoredReturn.searchParams.get("q")).toBe("香港");
  expect(restoredReturn.searchParams.get("status")).toBe("published");
  await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
});

it("redacts nested secrets while still identifying changed secret fields", () => {
  const value = { tls: { private_key: "private", certificate: "cert" }, routes: [{ password: "nested" }], server_port: 443 };
  const redacted = JSON.stringify(redactRevision(value));
  expect(redacted).not.toContain('"private"'); expect(redacted).not.toContain('"cert"'); expect(redacted).not.toContain('"nested"');
  expect(revisionDiff({ password: "a" }, { password: "b" })).toEqual([{ path: "password", before: "[已隐藏]", after: "[已隐藏]" }]);
});
