import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { Link, Route, Switch } from "wouter";
import { useState } from "react";
import { AdminNavigation, useUnsavedChanges } from "./AdminNavigation";
import { AdminThemeProvider, AdminThemeToggle } from "./AdminTheme";
import { safeReturnTo } from "../navigation";

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

it("defaults the authenticated console to light and retains an explicit dark preference", async () => {
  const view = render(<AdminThemeProvider><AdminThemeToggle /></AdminThemeProvider>);
  expect(document.documentElement.dataset.adminTheme).toBe("light");
  fireEvent.click(screen.getByRole("button", { name: "切换为深色模式" }));
  expect(localStorage.getItem("cpanel.theme")).toBe("dark");
  view.unmount();
  expect(document.documentElement.dataset.adminTheme).toBeUndefined();
  render(<AdminThemeProvider><AdminThemeToggle /></AdminThemeProvider>);
  await waitFor(() => expect(document.documentElement.dataset.adminTheme).toBe("dark"));
});

it("rejects external and cross-resource editor return targets", () => {
  expect(safeReturnTo("/machines", "https://elsewhere.example/machines")).toMatch(/^\/machines(?:\?|$)/);
  expect(safeReturnTo("/machines", "/users?q=other")).toMatch(/^\/machines(?:\?|$)/);
  expect(safeReturnTo("/machines", "/machines?q=hongkong&region=HK")).toContain("q=hongkong");
  vi.restoreAllMocks();
});
