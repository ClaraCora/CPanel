import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError, api } from "../api";
import { AdminNavigation } from "../components/AdminNavigation";
import { ToastProvider } from "../components/ui";
import { demoPlans } from "../mock";
import type { ResourceDefinition } from "../resourceDefinitions";
import type { Plan } from "../types";
import { ResourceListPage } from "./ResourceListPage";

let current: Plan;
const definition: ResourceDefinition<Plan> = {
  title: "套餐", description: "套餐配置", endpoint: "/plans", createLabel: "创建套餐",
  columns: [{ label: "名称", render: (item) => item.name }], fields: [], defaults: {},
  toValues: () => ({}), transform: (values) => values,
  identifierAction: { label: "ID", value: (item) => item.id },
};

beforeEach(() => {
  vi.restoreAllMocks();
  history.replaceState({}, "", "/plans");
  current = { ...demoPlans[0], status: "active", config_version: 7 };
  vi.spyOn(api, "get").mockImplementation(async () => [current] as never);
});

function open() { return render(<ToastProvider><AdminNavigation><ResourceListPage {...definition} /></AdminNavigation></ToastProvider>); }
function menu() { fireEvent.click(screen.getByRole("button", { name: `${current.id} 更多操作` })); }
function confirm(label: string) { fireEvent.click(within(screen.getByRole("alertdialog")).getByRole("button", { name: label })); }

describe("shared resource list write protection", () => {
  it("disables status and delete without a valid version while preserving edit and copy", async () => {
    current.config_version = undefined;
    const patch = vi.spyOn(api, "patch"), remove = vi.spyOn(api, "delete");
    open(); await screen.findByRole("table"); menu();
    expect((screen.getByRole("menuitem", { name: "停用" }) as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByRole("menuitem", { name: "删除" }) as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByRole("menuitem", { name: "编辑" }) as HTMLButtonElement).disabled).toBe(false);
    expect((screen.getByRole("menuitem", { name: "复制 ID" }) as HTMLButtonElement).disabled).toBe(false);
    expect(screen.getByRole("button", { name: "重新加载列表" })).toBeTruthy();
    expect(patch).not.toHaveBeenCalled(); expect(remove).not.toHaveBeenCalled();
  });

  it("uses the record version captured when each confirmation opens", async () => {
    const patch = vi.spyOn(api, "patch").mockImplementation(async () => { current = { ...current, status: "disabled", config_version: 9 }; return current as never; });
    const remove = vi.spyOn(api, "delete").mockResolvedValue({} as never);
    open(); await screen.findByRole("table"); menu();
    fireEvent.click(screen.getByRole("menuitem", { name: "停用" }));
    current.config_version = 8;
    confirm("确认停用");
    await waitFor(() => expect(patch).toHaveBeenCalledWith(`/plans/${current.id}`, { status: "disabled" }, { configVersion: 7 }));
    await waitFor(() => expect(screen.queryByRole("alertdialog")).toBeNull());
    await screen.findByRole("table"); menu();
    fireEvent.click(screen.getByRole("menuitem", { name: "删除" }));
    current.config_version = 10;
    confirm("确认删除");
    await waitFor(() => expect(remove).toHaveBeenCalledWith(`/plans/${current.id}`, { configVersion: 9 }));
  });

  it("closes stale confirmations and refreshes after conflicts without retrying, retaining diagnostics", async () => {
    const patch = vi.spyOn(api, "patch").mockImplementation(async () => {
      current = { ...current, config_version: 8 };
      throw new ApiError(409, { code: "EDIT_VERSION_CONFLICT", message: "配置已变化" }, "status-conflict-request");
    });
    const remove = vi.spyOn(api, "delete").mockImplementation(async () => {
      current = { ...current, config_version: 9 };
      throw new ApiError(409, { code: "CONFIG_VERSION_CONFLICT", message: "删除版本冲突" }, "delete-conflict-request");
    });
    open(); await screen.findByRole("table"); menu();
    fireEvent.click(screen.getByRole("menuitem", { name: "停用" })); confirm("确认停用");
    await screen.findByText("status-conflict-request");
    expect(screen.queryByRole("alertdialog")).toBeNull();
    await screen.findByRole("table");
    expect(screen.getByText("资源配置已变化。请检查重新加载后的列表，再次确认操作。")).toBeTruthy();
    expect(patch).toHaveBeenCalledTimes(1);
    menu(); fireEvent.click(screen.getByRole("menuitem", { name: "删除" })); confirm("确认删除");
    await screen.findByText("delete-conflict-request");
    expect(screen.queryByRole("alertdialog")).toBeNull();
    await screen.findByRole("table");
    expect(remove).toHaveBeenCalledTimes(1);
    expect(remove).toHaveBeenCalledWith(`/plans/${current.id}`, { configVersion: 8 });
    expect(api.get).toHaveBeenCalledTimes(3);
  });
});
