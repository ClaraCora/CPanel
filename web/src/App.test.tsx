import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";
import { api } from "./api";
import { demoSession } from "./mock";

vi.mock("./AuthenticatedApp", async () => {
  const { useState } = await import("react");
  return { default: function Editor() {
    const [value, setValue] = useState("原始配置");
    return <label>测试配置<input value={value} onChange={(event) => setValue(event.target.value)} /></label>;
  } };
});

function response(data: unknown, status = 200) {
  return new Response(JSON.stringify(status === 200 ? { data } : { error: { code: "ADMIN_SESSION_INVALID", message: "会话过期" }, meta: { request_id: "reauth-request" } }), { status, headers: { "Content-Type": "application/json" } });
}

describe("authentication recovery", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    history.replaceState({}, "", "/machines/mch_1/edit?returnTo=%2Fmachines%3Fq%3Dhk");
  });

  it("retains the route and unsaved input across same-administrator login without replaying a mutation", async () => {
    const fetch = vi.spyOn(globalThis, "fetch").mockImplementation(async (_url, init) => init?.method === "PATCH" ? response(null, 401) : response(demoSession));
    render(<App />);
    fireEvent.change(await screen.findByLabelText("测试配置"), { target: { value: "很长且尚未保存的配置" } });
    await act(async () => { await api.patch("/machines/mch_1", { name: "配置" }).catch(() => undefined); });
    await screen.findByRole("button", { name: "登录后台" });
    expect(location.pathname).toBe("/machines/mch_1/edit");
    const calls = fetch.mock.calls.length;
    await expect(api.get("/nodes")).rejects.toMatchObject({ status: 401 });
    expect(fetch.mock.calls.length).toBe(calls);
    fireEvent.change(screen.getByRole("textbox", { name: /邮箱/ }), { target: { value: "admin@cpanel.local" } });
    fireEvent.change(screen.getByLabelText(/^密码/), { target: { value: "password-for-login" } });
    fireEvent.click(screen.getByRole("button", { name: "登录后台" }));
    await waitFor(() => expect(screen.queryByRole("button", { name: "登录后台" })).toBeNull());
    expect((screen.getByRole("textbox", { name: "测试配置" }) as HTMLInputElement).value).toBe("很长且尚未保存的配置");
    expect(fetch.mock.calls.filter(([, init]) => init?.method === "PATCH")).toHaveLength(1);
  });

  it("does not give the previous draft to a different administrator", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (_url, init) => init?.method === "PATCH" ? response(null, 401) : response(init?.method === "POST" ? { ...demoSession, admin: { ...demoSession.admin, id: "another-admin" } } : demoSession));
    render(<App />);
    fireEvent.change(await screen.findByLabelText("测试配置"), { target: { value: "前一个管理员的配置" } });
    await act(async () => { await api.patch("/machines/mch_1", {}).catch(() => undefined); });
    fireEvent.change(await screen.findByRole("textbox", { name: /邮箱/ }), { target: { value: "other@cpanel.local" } });
    fireEvent.change(screen.getByLabelText(/^密码/), { target: { value: "password-for-login" } });
    fireEvent.click(screen.getByRole("button", { name: "登录后台" }));
    await waitFor(() => expect(screen.queryByRole("button", { name: "登录后台" })).toBeNull());
    expect((screen.getByRole("textbox", { name: "测试配置" }) as HTMLInputElement).value).toBe("原始配置");
    expect(location.pathname).toBe("/");
  });
});
