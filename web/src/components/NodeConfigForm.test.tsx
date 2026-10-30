import { useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { defaultNodeSettings, NodeConfigForm, type NodeSettings } from "./NodeConfigForm";

function Example({ initialProtocol = "vless", initialSettings = defaultNodeSettings, kernel = "xray" }: { initialProtocol?: string; initialSettings?: NodeSettings; kernel?: string }) {
  const [protocol, setProtocol] = useState(initialProtocol);
  const [settings, setSettings] = useState(initialSettings);
  return <><NodeConfigForm protocol={protocol} kernel={kernel} settings={settings} onProtocolChange={setProtocol} onSettingsChange={setSettings} /><output data-testid="settings">{JSON.stringify(settings)}</output></>;
}

describe("Hysteria2 TLS controls", () => {
  it("defaults to automatic self-signed certificates and persists checkbox changes", () => {
    render(<Example />);
    fireEvent.change(screen.getByLabelText("节点类型"), { target: { value: "hysteria" } });
    const skip = screen.getByRole("checkbox", { name: /跳过证书验证/ });
    expect((skip as HTMLInputElement).checked).toBe(true);
    const original = JSON.parse(screen.getByTestId("settings").textContent!);
    expect(original.cert_config.domain).toBe(original.tls.server_name);
    fireEvent.click(skip);
    expect(JSON.parse(screen.getByTestId("settings").textContent!).tls.insecure).toBe(false);
    fireEvent.click(screen.getByRole("button", { name: "随机生成名称" }));
    const regenerated = JSON.parse(screen.getByTestId("settings").textContent!);
    expect(regenerated.tls.server_name).not.toBe(original.tls.server_name);
    expect(regenerated.cert_config.domain).toBe(regenerated.tls.server_name);
    expect(regenerated.tls.insecure).toBe(false);
  });

  it("disables skip verification when switching to CA and enables it for self-signed mode", () => {
    render(<Example initialProtocol="hysteria" initialSettings={{ version: 2, tls: { enabled: true, server_name: "node.example.com", insecure: true }, cert_config: { cert_mode: "self" } }} />);
    const modes = screen.getByRole("option", { name: "自动生成自签证书" }).closest("select")!;
    fireEvent.change(modes, { target: { value: "http" } });
    expect((screen.getByRole("checkbox", { name: /跳过证书验证/ }) as HTMLInputElement).checked).toBe(false);
    fireEvent.change(modes, { target: { value: "self" } });
    const saved = JSON.parse(screen.getByTestId("settings").textContent!);
    expect(saved.tls.insecure).toBe(true);
    expect(saved.cert_config.domain).toBe("node.example.com");
  });

  it("prepares self-signed defaults when switching Hysteria 1 to Hysteria 2", () => {
    render(<Example kernel="singbox" initialProtocol="hysteria" initialSettings={{ version: 1, cert_config: { cert_mode: "self" } }} />);
    fireEvent.change(screen.getByLabelText("协议版本"), { target: { value: "2" } });
    const saved = JSON.parse(screen.getByTestId("settings").textContent!);
    expect(saved.tls.server_name).toMatch(/^edge-[0-9a-f]{16}\.example\.com$/);
    expect(saved.cert_config.domain).toBe(saved.tls.server_name);
    expect(saved.tls.insecure).toBe(true);
  });
});
