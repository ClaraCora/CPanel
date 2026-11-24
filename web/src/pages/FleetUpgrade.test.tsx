import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { api } from "../api";
import { demoMachines } from "../mock";
import type { FleetBatch } from "../types";
import { renderResource } from "../test/renderResource";
import { MachinesPage } from "./ResourcePages";

it("confirms the whole fleet through a filtered list and retries the failed machine", async () => {
  const machines = demoMachines.map(m => ({ ...m, latest_agent_version: "v2.1" }));
  const batch: FleetBatch = { id: "test-batch", created_at: "2026-11-24T09:00:00Z", target_version: "v2.1", items: machines.map((m, i) => ({
    machine_id: m.id, machine_name: m.name, agent_version: i === 0 ? "v2.1+abc" : m.agent_version,
    status: i === 0 ? "succeeded" : i === 1 ? "failed" : "waiting_online", updated_at: "2026-11-24T09:02:00Z",
    error: i === 1 ? "下载超时" : "", task_id: `task-${i}`,
  })) };
  vi.spyOn(api, "get").mockImplementation(async url => (url === "/machines" ? machines : url.endsWith("latest") ? null : batch) as never);
  const post = vi.spyOn(api, "post").mockImplementation(async url => {
    if (url.endsWith("/retry")) batch.items[1] = {...batch.items[1], status: "queued", error: ""};
    return structuredClone(batch) as never;
  });
  renderResource(MachinesPage, "/machines");
  const cards = await screen.findByRole("region", {name: "服务器指标对照"});
  expect(within(cards).getAllByRole("article").map((row) => row.getAttribute("aria-label"))).toEqual(machines.map((machine) => machine.name));
  fireEvent.click(screen.getByRole("button", { name: "隐藏地址" }));
  expect(within(cards).queryByText(machines[0].host)).toBeNull();
  fireEvent.change(screen.getByRole("textbox", { name: "搜索服务器" }), {target: {value: "香港"}});
  expect(within(cards).queryByText(machines[2].name)).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "升级全部 Agent" }));
  const confirm = await screen.findByRole("dialog", { name: "确认升级全部 Agent" });
  expect(within(confirm).getByText(machines[2].name)).toBeTruthy();
  fireEvent.click(within(confirm).getByRole("button", { name: "升级全部 3 台" }));
  const progress = await screen.findByRole("dialog", { name: "升级进度" });
  expect(within(progress).getByText("1 / 3 已达目标")).toBeTruthy();
  expect(within(progress).getByText("下载超时")).toBeTruthy();
  expect(post).toHaveBeenCalledWith("/agent-upgrade-batches", {target_version: "v2.1"}, {idempotencyKey: expect.any(String)});
  fireEvent.click(within(progress).getByRole("button", { name: `重试 ${machines[1].name}` }));
  await waitFor(() => expect(within(progress).getByText("等待下发")).toBeTruthy());
  expect(post).toHaveBeenLastCalledWith(`/agent-upgrade-batches/test-batch/machines/${machines[1].id}/retry`, {}, {idempotencyKey: expect.any(String)});
  expect(within(progress).getByText("待上线")).toBeTruthy();
});
