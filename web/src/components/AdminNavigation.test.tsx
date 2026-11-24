import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { Link, Route, Switch } from "wouter";
import { useState } from "react";
import { AdminNavigation, useUnsavedChanges } from "./AdminNavigation";
import { AdminThemeProvider, AdminThemeToggle } from "./AdminTheme";
import { AppShell } from "./AppShell";
import { safeReturnTo } from "../navigation";
import { initializeTheme } from "../theme";

beforeEach(() => { history.replaceState({}, "", "/edit"); localStorage.clear(); });

function Form() {
  const [value, setValue] = useState("");
  const bypass = useUnsavedChanges(Boolean(value));
  return <><label>配置名称<input value={value} onChange={(event) => setValue(event.target.value)} /></label><Link to="/list">返回列表</Link><button onClick={() => bypass(() => history.replaceState({}, "", "/list"))}>保存</button></>;
}

it("keeps the edited page and its draft until the user confirms leaving", async () => {
  render(<AdminNavigation><Switch><Route path="/edit" component={Form} /><Route path="/list"><h1>列表</h1></Route></Switch></AdminNavigation>);
  fireEvent.change(screen.getByLabelText("配置名称"), { target: { value: "未保存输入" } });
  fireEvent.click(screen.getByRole("link", { name: "返回列表" }));
  expect(location.pathname).toBe("/edit");
  fireEvent.click(screen.getByRole("button", { name: "继续编辑" }));
  expect((screen.getByLabelText("配置名称") as HTMLInputElement).value).toBe("未保存输入");
  fireEvent.click(screen.getByRole("link", { name: "返回列表" }));
  fireEvent.click(screen.getByRole("button", { name: "放弃修改并离开" }));
  await screen.findByRole("heading", { name: "列表" });
  expect(location.pathname).toBe("/list");
});

it("uses the native unload warning for a draft and permits a completed save", async () => {
  render(<AdminNavigation><Switch><Route path="/edit" component={Form} /><Route path="/list"><h1>列表</h1></Route></Switch></AdminNavigation>);
  fireEvent.change(screen.getByLabelText("配置名称"), { target: { value: "草稿" } });
  const event = new Event("beforeunload", { cancelable: true });
  window.dispatchEvent(event);
  expect(event.defaultPrevented).toBe(true);
  fireEvent.click(screen.getByRole("button", { name: "保存" }));
  await screen.findByRole("heading", { name: "列表" });
  expect(screen.queryByRole("alertdialog")).toBeNull();
});

it("starts the plans layout in light mode and keeps the chosen theme across layouts and reloads", async () => {
  history.replaceState({}, "", "/plans");
  initializeTheme();
  const view = render(<AdminThemeProvider><AdminThemeToggle /></AdminThemeProvider>);
  expect(document.documentElement.dataset.consoleLayout).toBe("B");
  expect(document.documentElement.dataset.adminTheme).toBe("light");
  fireEvent.click(screen.getByRole("button", { name: "切换为深色模式" }));
  expect(localStorage.getItem("cpanel.theme")).toBe("dark");
  expect(document.documentElement.dataset.theme).toBe("dark");
  view.unmount();
  expect(document.documentElement.dataset.adminTheme).toBeUndefined();
  history.replaceState({}, "", "/settings");
  initializeTheme();
  render(<AdminThemeProvider><AdminThemeToggle /></AdminThemeProvider>);
  expect(document.documentElement.dataset.consoleLayout).toBe("C");
  await waitFor(() => expect(document.documentElement.dataset.adminTheme).toBe("dark"));
});

function renderShell() {
  const admin = { id: "demo", name: "管理员", email: "", status: "active" };
  return render(<AdminNavigation><AdminThemeProvider><AppShell admin={admin} demo={false} onLogout={() => undefined}>
    <Route path="/edit" component={Form} />
  </AppShell></AdminThemeProvider></AdminNavigation>);
}

it("includes every desktop destination in the drawer and marks the active route", () => {
  history.replaceState({}, "", "/plans");
  renderShell();
  const nav = within(screen.getByRole("navigation", { name: "主导航" }));
  const destinations = nav.getAllByRole("link").map((link) => link.getAttribute("href"));
  for (const group of nav.getAllByRole("button")) {
    fireEvent.click(group);
    const items = within(screen.getByRole("menu")).getAllByRole("menuitem");
    destinations.push(...items.map((item) => item.getAttribute("href")));
    fireEvent.keyDown(document, { key: "Escape" });
  }
  fireEvent.click(screen.getByRole("button", { name: "打开导航" }));
  const drawer = within(screen.getByRole("dialog", { name: "主导航" }));
  expect(destinations).toHaveLength(13);
  expect(drawer.getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual(destinations);
  expect(drawer.getByRole("link", { current: "page" }).textContent).toBe("套餐");
  fireEvent.click(drawer.getByRole("link", { name: "系统设置" }));
  expect(location.pathname).toBe("/settings");
  expect(screen.queryByRole("dialog")).toBeNull();
});

it("closes a desktop submenu whose trigger was hidden on resize and focuses the drawer toggle", () => {
  renderShell();
  const nav = within(screen.getByRole("navigation", { name: "主导航" }));
  const group = nav.getByRole("button", { name: "账号与订阅" });
  fireEvent.click(group);
  within(screen.getByRole("menu")).getByRole("menuitem", { name: "套餐" }).focus();
  // In jsdom every element has an empty client rect, as the hidden trigger does in a narrow viewport.
  fireEvent(window, new Event("resize"));
  expect(screen.queryByRole("menu")).toBeNull();
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "打开导航" }));
});

it("contains keyboard focus, restores it on dismissal and unlocks page scrolling", () => {
  renderShell();
  const trigger = screen.getByRole("button", { name: "打开导航" });
  fireEvent.click(trigger);
  const drawer = within(screen.getByRole("dialog", { name: "主导航" }));
  const close = drawer.getByRole("button", { name: "关闭对话框" });
  expect(document.activeElement).toBe(close);
  expect(document.body.style.overflow).toBe("hidden");
  fireEvent.keyDown(close, { key: "Tab", shiftKey: true });
  expect(document.activeElement).toBe(drawer.getByRole("link", { name: "审计日志" }));
  fireEvent.keyDown(document.activeElement!, { key: "Tab" });
  expect(document.activeElement).toBe(close);
  fireEvent.keyDown(close, { key: "Escape" });
  expect(document.activeElement).toBe(trigger);
  expect(trigger.getAttribute("aria-expanded")).toBe("false");
  fireEvent.click(trigger);
  fireEvent.click(screen.getByRole("button", { name: "关闭对话框背景" }));
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(document.activeElement).toBe(trigger);
  expect(document.body.style.overflow).toBe("");
});

it("closes the drawer before requesting confirmation for an unsaved page", () => {
  renderShell();
  fireEvent.change(screen.getByLabelText("配置名称"), { target: { value: "草稿" } });
  const trigger = screen.getByRole("button", { name: "打开导航" });
  function selectPlans() {
    fireEvent.click(trigger);
    fireEvent.click(within(screen.getByRole("dialog", { name: "主导航" })).getByRole("link", { name: "套餐" }));
  }
  selectPlans();
  expect(screen.queryByRole("dialog")).toBeNull();
  const keepEditing = screen.getByRole("button", { name: "继续编辑" });
  expect(document.activeElement).toBe(keepEditing);
  fireEvent.click(keepEditing);
  expect(location.pathname).toBe("/edit");
  expect((screen.getByLabelText("配置名称") as HTMLInputElement).value).toBe("草稿");
  selectPlans();
  fireEvent.click(screen.getByRole("button", { name: "放弃修改并离开" }));
  expect(location.pathname).toBe("/plans");
  expect(document.body.style.overflow).toBe("");
});

it("rejects external and cross-resource editor return targets", () => {
  expect(safeReturnTo("/machines", "https://elsewhere.example/machines")).toMatch(/^\/machines(?:\?|$)/);
  expect(safeReturnTo("/machines", "/users?q=other")).toMatch(/^\/machines(?:\?|$)/);
  expect(safeReturnTo("/machines", "/machines?q=hongkong&region=HK")).toContain("q=hongkong");
  vi.restoreAllMocks();
});
