type Settings = Record<string, unknown>;

function object(value: unknown): Settings {
  return value && typeof value === "object" && !Array.isArray(value) ? value as Settings : {};
}

export function isHysteria2(protocol: string, settings: Settings) {
  return protocol === "hysteria2" || (protocol === "hysteria" && Number(settings.version ?? 2) === 2);
}

export function isSelfSignedHysteria2(protocol: string, settings: Settings) {
  return isHysteria2(protocol, settings) && (object(settings.cert_config).cert_mode ?? "self") === "self";
}

export function skipCertificateVerification(protocol: string, settings: Settings) {
  const insecure = object(settings.tls).insecure;
  return typeof insecure === "boolean" ? insecure : isSelfSignedHysteria2(protocol, settings);
}

export function randomTLSServerName() {
  const bytes = new Uint8Array(8);
  crypto.getRandomValues(bytes);
  // A reserved domain is sufficient for a self-signed SNI; no DNS or ownership is required.
  return `edge-${Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("")}.example.com`;
}

export function prepareHysteriaTLS(protocol: string, settings: Settings): Settings {
  if (!isSelfSignedHysteria2(protocol, settings)) return settings;
  const tls = object(settings.tls);
  const cert = object(settings.cert_config);
  const serverName = (typeof tls.server_name === "string" && tls.server_name.trim())
    || (typeof cert.domain === "string" && cert.domain.trim()) || randomTLSServerName();
  return {
    ...settings,
    tls: { ...tls, enabled: true, server_name: serverName, insecure: skipCertificateVerification(protocol, settings) },
    cert_config: { ...cert, cert_mode: "self", domain: serverName },
  };
}
