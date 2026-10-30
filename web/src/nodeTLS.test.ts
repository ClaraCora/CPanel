import { describe, expect, it } from "vitest";
import { prepareHysteriaTLS, randomTLSServerName, skipCertificateVerification } from "./nodeTLS";

describe("Hysteria2 self-signed TLS settings", () => {
  it("generates a stable SNI, matching certificate domain and client default", () => {
    const settings = prepareHysteriaTLS("hysteria", { version: 2 });
    const tls = settings.tls as Record<string, unknown>;
    expect(tls.server_name).toMatch(/^edge-[0-9a-f]{16}\.example\.com$/);
    expect(tls.enabled).toBe(true);
    expect(tls.insecure).toBe(true);
    expect(settings.cert_config).toEqual({ cert_mode: "self", domain: tls.server_name });
    expect(prepareHysteriaTLS("hysteria", settings)).toEqual(settings);
  });

  it("preserves explicit opt-out and existing certificate options", () => {
    const settings = prepareHysteriaTLS("hysteria2", {
      tls: { server_name: "node.example.com", insecure: false },
      cert_config: { cert_mode: "self", domain: "old.example.com", http_port: 80 },
    });
    expect(settings.tls).toEqual({ enabled: true, server_name: "node.example.com", insecure: false });
    expect(settings.cert_config).toEqual({ cert_mode: "self", domain: "node.example.com", http_port: 80 });
    expect(skipCertificateVerification("hysteria2", settings)).toBe(false);
  });

  it("does not change CA, file, upstream, other protocol or Hysteria1 settings", () => {
    for (const mode of ["http", "dns", "file", "content", "none"]) {
      const settings = { cert_config: { cert_mode: mode }, tls: { enabled: true } };
      expect(prepareHysteriaTLS("hysteria", settings)).toBe(settings);
      expect(skipCertificateVerification("hysteria", settings)).toBe(false);
    }
    const settings = { version: 1, cert_config: { cert_mode: "self" } };
    expect(prepareHysteriaTLS("hysteria", settings)).toBe(settings);
    expect(prepareHysteriaTLS("vless", settings)).toBe(settings);
    expect(randomTLSServerName()).not.toEqual(randomTLSServerName());
  });
});
