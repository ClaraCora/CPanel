import type { ComponentType } from "react";
import { Cable, Cloud, KeyRound, LockKeyhole, Network, Radio, RefreshCw, ShieldCheck } from "lucide-react";
import type { Outbound } from "../types";
import { Button, Field } from "./ui";

export type OutboundSettings = Record<string, unknown>;

type ProtocolOption = {
  id: string;
  label: string;
  description: string;
  defaultPort: number;
  icon: ComponentType<{ size?: number; "aria-hidden"?: boolean }>;
};

export const outboundProtocolOptions: ProtocolOption[] = [
  { id: "vless", label: "VLESS", description: "UUID 认证出站", defaultPort: 443, icon: ShieldCheck },
  { id: "vmess", label: "VMess", description: "VMess 用户认证出站", defaultPort: 443, icon: Network },
  { id: "trojan", label: "Trojan", description: "密码认证出站", defaultPort: 443, icon: LockKeyhole },
  { id: "shadowsocks", label: "Shadowsocks", description: "AEAD 加密代理出站", defaultPort: 8388, icon: KeyRound },
  { id: "socks", label: "SOCKS", description: "SOCKS5 上游代理", defaultPort: 1080, icon: Cable },
  { id: "http", label: "HTTP", description: "HTTP 上游代理", defaultPort: 8080, icon: Cloud },
  { id: "wireguard", label: "WireGuard", description: "WireGuard 隧道出站", defaultPort: 2408, icon: Radio },
];

const serverListProtocols = new Set(["trojan", "shadowsocks", "socks", "http"]);
const uuidProtocols = new Set(["vless", "vmess"]);

export function defaultOutboundSettings(protocol: string): OutboundSettings {
  const port = outboundProtocolOptions.find((item) => item.id === protocol)?.defaultPort ?? 443;
  switch (protocol) {
    case "vless":
      return { vnext: [{ address: "", port, users: [{ id: "", encryption: "none" }] }] };
    case "vmess":
      return { vnext: [{ address: "", port, users: [{ id: "", security: "auto", alterId: 0 }] }] };
    case "trojan":
      return { servers: [{ address: "", port, password: "" }] };
    case "shadowsocks":
      return { servers: [{ address: "", port, method: "aes-128-gcm", password: "" }] };
    case "socks":
    case "http":
      return { servers: [{ address: "", port, users: [] }] };
    case "wireguard":
      return { secretKey: "", address: [], peers: [{ publicKey: "", endpoint: "", allowedIPs: ["0.0.0.0/0", "::/0"] }], mtu: 1420, domainStrategy: "ForceIP" };
    default:
      return {};
  }
}

export function parseOutboundSettings(value: string): OutboundSettings {
  try {
    const parsed = JSON.parse(value || "{}");
    return objectValue(parsed);
  } catch {
    return {};
  }
}

export function validateOutboundSettingsValue(protocol: string, value: string): string {
  let settings: OutboundSettings;
  try {
    settings = objectValue(JSON.parse(value || "{}"));
  } catch {
    return "出站参数无效，请重新填写";
  }
  if (!outboundProtocolOptions.some((item) => item.id === protocol)) return "请选择有效的出站协议";
  if (protocol === "wireguard") {
    const peer = firstObject(settings.peers);
    if (!stringValue(settings.secretKey)) return "请填写 WireGuard 私钥";
    if (stringList(settings.address).length === 0) return "请填写 WireGuard 本地地址";
    if (!stringValue(peer.publicKey)) return "请填写 WireGuard 对端公钥";
    if (!stringValue(peer.endpoint)) return "请填写 WireGuard 对端地址";
    return "";
  }
  const server = firstServer(settings, protocol);
  if (!stringValue(server.address)) return "请填写服务器地址";
  const port = numberValue(server.port);
  if (port < 1 || port > 65535) return "服务器端口必须在 1 到 65535 之间";
  if (uuidProtocols.has(protocol)) {
    const id = stringValue(firstUser(server).id);
    if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(id)) return "请填写有效的 UUID";
  }
  if ((protocol === "trojan" || protocol === "shadowsocks") && !stringValue(server.password)) return "请填写认证密码";
  if (protocol === "socks" || protocol === "http") {
    const user = firstUser(server);
    if (Boolean(stringValue(user.user)) !== Boolean(stringValue(user.pass))) return "用户名和密码需要同时填写或同时留空";
  }
  return "";
}

export function OutboundConfigForm({ protocol, settings, proxyTag, outbounds, currentID, onProtocolChange, onSettingsChange, onProxyTagChange }: {
  protocol: string;
  settings: OutboundSettings;
  proxyTag: string;
  outbounds: Outbound[];
  currentID?: string;
  onProtocolChange: (protocol: string, settings: OutboundSettings) => void;
  onSettingsChange: (settings: OutboundSettings) => void;
  onProxyTagChange: (value: string) => void;
}) {
  const currentProtocol = outboundProtocolOptions.find((item) => item.id === protocol) ?? outboundProtocolOptions[0];
  const CurrentIcon = currentProtocol.icon;
  const server = firstServer(settings, protocol);
  const user = firstUser(server);
  const peer = firstObject(settings.peers);

  function changeProtocol(nextProtocol: string) {
    const next = defaultOutboundSettings(nextProtocol);
    if (protocol !== "wireguard" && nextProtocol !== "wireguard") {
      const previous = firstServer(settings, protocol);
      if (stringValue(previous.address)) {
        setServerMutable(next, nextProtocol, "address", previous.address);
        if (numberValue(previous.port)) setServerMutable(next, nextProtocol, "port", previous.port);
      }
    }
    onProtocolChange(nextProtocol, next);
  }

  function setServer(key: string, value: unknown) {
    const next = structuredClone(settings);
    setServerMutable(next, protocol, key, value);
    onSettingsChange(next);
  }

  function setUser(key: string, value: unknown) {
    const next = structuredClone(settings);
    const target = firstServerMutable(next, protocol);
    const users = arrayValue(target.users).map((item) => objectValue(item));
    const updated = { ...(users[0] ?? {}) };
    if (value === "") delete updated[key]; else updated[key] = value;
    target.users = [updated, ...users.slice(1)];
    onSettingsChange(next);
  }

  function setRoot(key: string, value: unknown) {
    const next = { ...settings };
    if (value === "" || value === undefined) delete next[key]; else next[key] = value;
    onSettingsChange(next);
  }

  function setPeer(key: string, value: unknown) {
    const next = structuredClone(settings);
    const peers = arrayValue(next.peers).map((item) => objectValue(item));
    const updated = { ...(peers[0] ?? {}) };
    if (value === "" || value === undefined) delete updated[key]; else updated[key] = value;
    next.peers = [updated, ...peers.slice(1)];
    onSettingsChange(next);
  }

  return <div className="outbound-builder">
    <div className="protocol-selector outbound-protocol-selector">
      <div className="protocol-selector__field">
        <label htmlFor="outbound-protocol">出站协议</label>
        <select id="outbound-protocol" value={protocol} onChange={(event) => changeProtocol(event.target.value)}>
          {outboundProtocolOptions.map((option) => <option value={option.id} key={option.id}>{option.label}</option>)}
        </select>
      </div>
      <div className="protocol-selector__summary">
        <span className="protocol-selector__icon"><CurrentIcon size={18} aria-hidden={true} /></span>
        <span className="protocol-selector__copy"><strong>{currentProtocol.label}</strong><small>{currentProtocol.description}</small></span>
        <span className="protocol-selector__support">Xray</span>
      </div>
    </div>

    {protocol !== "wireguard" ? <div className="config-group">
      <div className="config-group__heading"><h3>连接参数</h3></div>
      <div className="form-grid form-grid--three config-fields">
        <Field label="服务器地址" required><input value={stringValue(server.address)} onChange={(event) => setServer("address", event.target.value.trim())} placeholder="proxy.example.com" /></Field>
        <Field label="服务器端口" required><input type="number" min="1" max="65535" value={numberValue(server.port, currentProtocol.defaultPort)} onChange={(event) => setServer("port", Number(event.target.value))} /></Field>
        {uuidProtocols.has(protocol) && <Field label="用户 UUID" required><div className="input-with-action"><input className="mono" value={stringValue(user.id)} onChange={(event) => setUser("id", event.target.value.trim())} /><Button type="button" onClick={() => setUser("id", crypto.randomUUID())}><RefreshCw size={15} aria-hidden="true" />生成</Button></div></Field>}
        {protocol === "vless" && <><Field label="加密方式"><select value={stringValue(user.encryption, "none")} onChange={(event) => setUser("encryption", event.target.value)}><option value="none">none</option></select></Field><Field label="流控"><select value={stringValue(user.flow)} onChange={(event) => setUser("flow", event.target.value)}><option value="">不启用</option><option value="xtls-rprx-vision">XTLS Vision</option></select></Field></>}
        {protocol === "vmess" && <><Field label="安全算法"><select value={stringValue(user.security, "auto")} onChange={(event) => setUser("security", event.target.value)}><option value="auto">auto</option><option value="aes-128-gcm">aes-128-gcm</option><option value="chacha20-poly1305">chacha20-poly1305</option><option value="none">none</option></select></Field><Field label="Alter ID"><input type="number" min="0" value={numberValue(user.alterId)} onChange={(event) => setUser("alterId", Number(event.target.value))} /></Field></>}
        {protocol === "trojan" && <Field label="认证密码" required><input className="mono" type="text" autoComplete="off" value={stringValue(server.password)} onChange={(event) => setServer("password", event.target.value)} /></Field>}
        {protocol === "shadowsocks" && <><Field label="加密方式" required><select value={stringValue(server.method, "aes-128-gcm")} onChange={(event) => setServer("method", event.target.value)}><option value="aes-128-gcm">aes-128-gcm</option><option value="aes-256-gcm">aes-256-gcm</option><option value="chacha20-poly1305">chacha20-poly1305</option><option value="2022-blake3-aes-128-gcm">2022-blake3-aes-128-gcm</option><option value="2022-blake3-aes-256-gcm">2022-blake3-aes-256-gcm</option></select></Field><Field label="认证密码" required><input className="mono" type="text" autoComplete="off" value={stringValue(server.password)} onChange={(event) => setServer("password", event.target.value)} /></Field></>}
        {(protocol === "socks" || protocol === "http") && <><Field label="用户名"><input value={stringValue(user.user)} onChange={(event) => setUser("user", event.target.value)} placeholder="可留空" /></Field><Field label="密码"><input className="mono" type="text" autoComplete="off" value={stringValue(user.pass)} onChange={(event) => setUser("pass", event.target.value)} placeholder="可留空" /></Field></>}
      </div>
    </div> : <div className="config-group">
      <div className="config-group__heading"><h3>WireGuard 参数</h3></div>
      <div className="form-grid form-grid--three config-fields">
        <Field label="本机私钥" required><input className="mono" type="text" autoComplete="off" value={stringValue(settings.secretKey)} onChange={(event) => setRoot("secretKey", event.target.value.trim())} /></Field>
        <Field label="本地地址" required helper="每行一个 CIDR"><textarea rows={3} value={stringList(settings.address).join("\n")} onChange={(event) => setRoot("address", splitLines(event.target.value))} placeholder={"172.16.0.2/32\n2606:4700:110:8765::2/128"} /></Field>
        <Field label="对端公钥" required><input className="mono" value={stringValue(peer.publicKey)} onChange={(event) => setPeer("publicKey", event.target.value.trim())} /></Field>
        <Field label="对端地址" required><input value={stringValue(peer.endpoint)} onChange={(event) => setPeer("endpoint", event.target.value.trim())} placeholder="engage.cloudflareclient.com:2408" /></Field>
        <Field label="预共享密钥"><input className="mono" type="text" autoComplete="off" value={stringValue(peer.preSharedKey)} onChange={(event) => setPeer("preSharedKey", event.target.value.trim())} /></Field>
        <Field label="允许地址" helper="每行一个 CIDR"><textarea rows={3} value={stringList(peer.allowedIPs).join("\n")} onChange={(event) => setPeer("allowedIPs", splitLines(event.target.value))} /></Field>
        <Field label="保持连接（秒）"><input type="number" min="0" value={numberValue(peer.keepAlive)} onChange={(event) => setPeer("keepAlive", Number(event.target.value))} /></Field>
        <Field label="MTU"><input type="number" min="576" max="9000" value={numberValue(settings.mtu, 1420)} onChange={(event) => setRoot("mtu", Number(event.target.value))} /></Field>
        <Field label="域名解析策略"><select value={stringValue(settings.domainStrategy, "ForceIP")} onChange={(event) => setRoot("domainStrategy", event.target.value)}><option value="ForceIP">ForceIP</option><option value="ForceIPv4">ForceIPv4</option><option value="ForceIPv6">ForceIPv6</option></select></Field>
      </div>
    </div>}

    <div className="config-group outbound-chain">
      <div className="config-group__heading"><h3>链式出站</h3></div>
      <div className="config-fields config-fields--single"><Field label="上游出站"><select value={proxyTag} onChange={(event) => onProxyTagChange(event.target.value)}><option value="">不使用上游</option>{outbounds.filter((item) => item.id !== currentID && item.status === "active" && item.protocol !== "direct" && item.protocol !== "block").map((item) => <option value={item.tag} key={item.id}>{item.name} · {item.tag}</option>)}</select></Field></div>
    </div>
  </div>;
}

function objectValue(value: unknown): OutboundSettings {
  return value && typeof value === "object" && !Array.isArray(value) ? value as OutboundSettings : {};
}

function arrayValue(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

function stringValue(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function numberValue(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function stringList(value: unknown): string[] {
  return arrayValue(value).filter((item): item is string => typeof item === "string");
}

function firstObject(value: unknown): OutboundSettings {
  return objectValue(arrayValue(value)[0]);
}

function firstServer(settings: OutboundSettings, protocol: string): OutboundSettings {
  return firstObject(settings[uuidProtocols.has(protocol) ? "vnext" : "servers"]);
}

function firstUser(server: OutboundSettings): OutboundSettings {
  return firstObject(server.users);
}

function firstServerMutable(settings: OutboundSettings, protocol: string): OutboundSettings {
  const key = uuidProtocols.has(protocol) ? "vnext" : serverListProtocols.has(protocol) ? "servers" : "servers";
  const list = arrayValue(settings[key]).map((item) => objectValue(item));
  const server = { ...(list[0] ?? {}) };
  settings[key] = [server, ...list.slice(1)];
  return server;
}

function setServerMutable(settings: OutboundSettings, protocol: string, key: string, value: unknown) {
  const server = firstServerMutable(settings, protocol);
  if (value === "" || value === undefined) delete server[key]; else server[key] = value;
}

function splitLines(value: string): string[] {
  return value.split(/[\r\n,，]+/).map((item) => item.trim()).filter(Boolean);
}
