import { useState, type ComponentType } from "react";
import {
  Activity,
  Cable,
  Cloud,
  Gauge,
  Globe2,
  KeyRound,
  Layers3,
  LockKeyhole,
  Network,
  Radio,
  RefreshCw,
  ShieldCheck,
} from "lucide-react";
import { api } from "../api";
import { Button, Field } from "./ui";

export type NodeSettings = Record<string, unknown>;

type ProtocolOption = {
  id: string;
  label: string;
  description: string;
  kernels: string[];
  icon: ComponentType<{ size?: number; "aria-hidden"?: boolean }>;
};

export const protocolOptions: ProtocolOption[] = [
  { id: "vless", label: "VLESS", description: "UUID 鉴权与 Reality", kernels: ["singbox", "xray"], icon: ShieldCheck },
  { id: "vmess", label: "VMess", description: "WebSocket 与 gRPC", kernels: ["singbox", "xray"], icon: Network },
  { id: "trojan", label: "Trojan", description: "TLS 密码协议", kernels: ["singbox", "xray"], icon: LockKeyhole },
  { id: "shadowsocks", label: "Shadowsocks", description: "AEAD 与 2022 加密", kernels: ["singbox", "xray"], icon: KeyRound },
  { id: "hysteria", label: "Hysteria", description: "Hysteria 1 / 2", kernels: ["singbox", "xray"], icon: Gauge },
  { id: "tuic", label: "TUIC", description: "QUIC 低延迟传输", kernels: ["singbox"], icon: Activity },
  { id: "anytls", label: "AnyTLS", description: "TLS 流量整形", kernels: ["singbox"], icon: Layers3 },
  { id: "naive", label: "Naive", description: "HTTPS 代理入站", kernels: ["singbox"], icon: Globe2 },
  { id: "mieru", label: "Mieru", description: "TCP / UDP 混淆", kernels: ["singbox"], icon: Radio },
  { id: "socks", label: "SOCKS", description: "通用 SOCKS5 代理", kernels: ["singbox", "xray"], icon: Cable },
  { id: "http", label: "HTTP", description: "HTTP 认证代理", kernels: ["singbox", "xray"], icon: Cloud },
];

const transportProtocols = new Set(["vless", "vmess", "trojan"]);
const tlsProtocols = new Set(["vless", "vmess", "trojan", "hysteria", "tuic", "anytls", "naive", "http"]);
const requiredTLSProtocols = new Set(["hysteria", "tuic", "anytls"]);
const realityProtocols = new Set(["vless", "trojan"]);
const multiplexProtocols = new Set(["vless", "vmess", "trojan"]);
const realityFingerprints = [
  { value: "chrome", label: "Chrome" },
  { value: "firefox", label: "Firefox" },
  { value: "safari", label: "Safari" },
  { value: "ios", label: "iOS" },
  { value: "android", label: "Android" },
  { value: "edge", label: "Edge" },
  { value: "360", label: "360 浏览器" },
  { value: "qq", label: "QQ 浏览器" },
  { value: "random", label: "随机指纹" },
];
const realityFingerprintValues = new Set(realityFingerprints.map((option) => option.value));
const ss2022KeySizes: Record<string, number> = {
  "2022-blake3-aes-128-gcm": 16,
  "2022-blake3-aes-256-gcm": 32,
};
const protocolSpecificKeys = [
  "cipher",
  "server_key",
  "flow",
  "decryption",
  "version",
  "up_mbps",
  "down_mbps",
  "obfs",
  "obfs_password",
  "obfs-password",
  "congestion_control",
  "padding_scheme",
  "traffic_pattern",
];

export const defaultNodeSettings: NodeSettings = {
  transport: "tcp",
  tls: { enabled: true },
  cert_config: { cert_mode: "self" },
};

export function protocolSupported(protocol: string, kernel: string) {
  return protocolOptions.some((option) => option.id === protocol && option.kernels.includes(kernel));
}

function objectValue(value: unknown): NodeSettings {
  return value && typeof value === "object" && !Array.isArray(value) ? value as NodeSettings : {};
}

function stringValue(value: unknown, fallback = "") {
  return typeof value === "string" ? value : fallback;
}

function numberValue(value: unknown, fallback = 0) {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function booleanValue(value: unknown) {
  return value === true || value === 1 || value === "1";
}

function setValue(settings: NodeSettings, key: string, value: unknown) {
  const next = { ...settings };
  if (value === "" || value === undefined || value === null) delete next[key];
  else next[key] = value;
  return next;
}

function setObjectValue(settings: NodeSettings, objectKey: string, key: string, value: unknown) {
  const child = { ...objectValue(settings[objectKey]) };
  if (value === "" || value === undefined || value === null) delete child[key];
  else child[key] = value;
  const next = { ...settings, [objectKey]: child };
  if (objectKey === "network_settings") delete next.networkSettings;
  return next;
}

function tlsMode(settings: NodeSettings, protocol: string) {
  if (requiredTLSProtocols.has(protocol)) return "tls";
  const tls = settings.tls;
  const enabled = objectValue(tls).enabled ?? tls;
  if (enabled === 2 || enabled === "2") return "reality";
  return booleanValue(enabled) ? "tls" : "off";
}

function withTLSMode(settings: NodeSettings, mode: string) {
  const tls: NodeSettings = { ...objectValue(settings.tls), enabled: mode === "reality" ? 2 : mode === "tls" };
  if (mode === "reality" && !stringValue(tls.fingerprint)) tls.fingerprint = "chrome";
  return { ...settings, tls };
}

function base64ByteLength(value: string) {
  const normalized = value.trim();
  if (!/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(normalized)) return -1;
  try {
    return window.atob(normalized).length;
  } catch {
    return -1;
  }
}

function ss2022ServerKeyError(cipher: string, serverKey: string) {
  const expectedBytes = ss2022KeySizes[cipher];
  if (!expectedBytes) return "";
  if (!serverKey.trim()) return `请生成或填写 ${expectedBytes} 字节的服务端密钥`;
  const actualBytes = base64ByteLength(serverKey);
  if (actualBytes < 0) return "服务端密钥必须是标准 Base64 编码";
  if (actualBytes !== expectedBytes) return `当前加密方式需要 ${expectedBytes} 字节密钥，实际为 ${actualBytes} 字节`;
  return "";
}

function randomSS2022ServerKey(cipher: string) {
  const bytes = new Uint8Array(ss2022KeySizes[cipher]);
  window.crypto.getRandomValues(bytes);
  return window.btoa(String.fromCharCode(...bytes));
}

function validX25519Key(value: string) {
  return /^[A-Za-z0-9_-]{43}$/.test(value);
}

type RealityCredentials = {
  private_key: string;
  public_key: string;
  short_id: string;
};

function withProtocolDefaults(settings: NodeSettings, protocol: string) {
  const next = { ...settings };
  protocolSpecificKeys.forEach((key) => delete next[key]);
  if (protocol === "shadowsocks") next.cipher = "aes-128-gcm";
  if (protocol === "vless") next.decryption = "none";
  if (protocol === "hysteria") next.version = 2;
  if (protocol === "tuic") next.congestion_control = "bbr";
  if (protocol === "mieru") next.transport = "tcp";
  if (requiredTLSProtocols.has(protocol)) {
    next.tls = { ...objectValue(next.tls), enabled: true };
    if (!Object.keys(objectValue(next.cert_config)).length) next.cert_config = { cert_mode: "self" };
  }
  return next;
}

export function validateNodeSettings(protocol: string, kernel: string, settings: NodeSettings) {
  if (!protocolSupported(protocol, kernel)) return `${protocol} 不支持 ${kernel === "xray" ? "Xray" : "sing-box"} 内核，请重新选择节点类型`;
  if (protocol === "shadowsocks" && !stringValue(settings.cipher)) return "请选择 Shadowsocks 加密方式";
  if (protocol === "shadowsocks") {
    const serverKeyError = ss2022ServerKeyError(stringValue(settings.cipher), stringValue(settings.server_key));
    if (serverKeyError) return serverKeyError;
  }
  if (tlsMode(settings, protocol) === "reality") {
    const tls = objectValue(settings.tls);
    const privateKey = stringValue(tls.private_key);
    const publicKey = stringValue(tls.public_key);
    if (!privateKey || !publicKey || !stringValue(tls.short_id) || !stringValue(tls.server_name) || !stringValue(tls.dest) || !stringValue(tls.fingerprint)) {
      return "Reality 需要填写私钥、公钥、Short ID、伪装域名、目标地址和客户端指纹";
    }
    if (!validX25519Key(privateKey) || !validX25519Key(publicKey)) return "Reality 私钥和公钥必须是有效的 X25519 Base64URL 密钥";
    if (!/^(?:[0-9a-fA-F]{2}){1,8}$/.test(stringValue(tls.short_id))) return "Short ID 必须是 2 到 16 位偶数长度的十六进制字符";
    if (!realityFingerprintValues.has(stringValue(tls.fingerprint))) return "请选择有效的客户端伪装指纹";
  }
  const cert = objectValue(settings.cert_config);
  if (tlsMode(settings, protocol) === "tls" && stringValue(cert.cert_mode) === "file" && (!stringValue(cert.cert_file) || !stringValue(cert.key_file))) {
    return "文件证书模式需要填写证书文件和私钥文件路径";
  }
  if (tlsMode(settings, protocol) === "tls" && ["http", "dns"].includes(stringValue(cert.cert_mode)) && !stringValue(cert.domain)) {
    return "ACME 证书模式需要填写证书域名";
  }
  return "";
}

export function NodeConfigForm({
  protocol,
  kernel,
  settings,
  error,
  onProtocolChange,
  onSettingsChange,
}: {
  protocol: string;
  kernel: string;
  settings: NodeSettings;
  error?: string;
  onProtocolChange: (value: string) => void;
  onSettingsChange: (value: NodeSettings) => void;
}) {
  const transport = stringValue(settings.transport, stringValue(settings.network, "tcp"));
  const networkSettings = objectValue(settings.network_settings ?? settings.networkSettings);
  const currentTLSMode = tlsMode(settings, protocol);
  const tls = objectValue(settings.tls);
  const cert = objectValue(settings.cert_config);
  const certMode = stringValue(cert.cert_mode, "none");
  const multiplex = objectValue(settings.multiplex);
  const cipher = stringValue(settings.cipher, "aes-128-gcm");
  const ss2022KeySize = ss2022KeySizes[cipher];
  const [realityGenerating, setRealityGenerating] = useState(false);
  const [credentialError, setCredentialError] = useState("");

  function update(value: NodeSettings) {
    onSettingsChange(value);
  }

  function updateTransport(value: string) {
    const next = setValue(settings, "transport", value);
    delete next.network;
    update(next);
  }

  function updateNetwork(key: string, value: unknown) {
    update(setObjectValue(settings, "network_settings", key, value));
  }

  function updateTLS(key: string, value: unknown) {
    update(setObjectValue(settings, "tls", key, value));
  }

  function updateCert(key: string, value: unknown) {
    update(setObjectValue(settings, "cert_config", key, value));
  }

  function updateCipher(value: string) {
    let next = setValue(settings, "cipher", value);
    const existingKey = stringValue(next.server_key);
    if (!ss2022KeySizes[value] || ss2022ServerKeyError(value, existingKey)) next = setValue(next, "server_key", "");
    update(next);
  }

  async function generateRealityCredentials() {
    setRealityGenerating(true);
    setCredentialError("");
    try {
      const credentials = await api.post<RealityCredentials>("/tools/reality-keypair");
      update({
        ...settings,
        tls: {
          ...objectValue(settings.tls),
          enabled: 2,
          fingerprint: stringValue(objectValue(settings.tls).fingerprint, "chrome"),
          ...credentials,
        },
      });
    } catch (reason) {
      setCredentialError(reason instanceof Error ? reason.message : "Reality 密钥生成失败，请重试");
    } finally {
      setRealityGenerating(false);
    }
  }

  return (
    <div className="node-config-form">
      <div className="protocol-picker" role="group" aria-label="节点类型">
        {protocolOptions.map((option) => {
          const selected = option.id === protocol;
          const supported = option.kernels.includes(kernel);
          const Icon = option.icon;
          return (
            <button
              type="button"
              className={`protocol-option ${selected ? "protocol-option--selected" : ""}`}
              aria-pressed={selected}
              disabled={!supported}
              title={supported ? `选择 ${option.label}` : `${option.label} 不支持当前内核`}
              onClick={() => {
                onProtocolChange(option.id);
                update(withProtocolDefaults(settings, option.id));
              }}
              key={option.id}
            >
              <span className="protocol-option__icon"><Icon size={18} aria-hidden={true} /></span>
              <span><strong>{option.label}</strong><small>{option.description}</small></span>
              {!supported && <em>仅 sing-box</em>}
            </button>
          );
        })}
      </div>
      {error && <div className="config-error" role="alert">{error}</div>}

      {transportProtocols.has(protocol) && (
        <div className="config-group">
          <div className="config-group__heading"><h3>传输方式</h3><p>选择客户端连接节点时使用的网络传输。</p></div>
          <div className="form-grid form-grid--three config-fields">
            <Field label="传输协议">
              <select value={transport} onChange={(event) => updateTransport(event.target.value)}>
                <option value="tcp">TCP</option>
                <option value="ws">WebSocket</option>
                <option value="grpc">gRPC</option>
                <option value="httpupgrade">HTTP Upgrade</option>
                <option value="h2">HTTP/2</option>
                {kernel === "xray" && <option value="xhttp">XHTTP</option>}
              </select>
            </Field>
            {transport === "grpc" ? (
              <Field label="服务名称"><input value={stringValue(networkSettings.service_name, stringValue(networkSettings.serviceName))} onChange={(event) => updateNetwork("service_name", event.target.value)} placeholder="grpc-service" /></Field>
            ) : transport !== "tcp" ? (
              <>
                <Field label="路径"><input value={stringValue(networkSettings.path)} onChange={(event) => updateNetwork("path", event.target.value)} placeholder="/path" /></Field>
                <Field label="Host"><input value={stringValue(networkSettings.host)} onChange={(event) => updateNetwork("host", event.target.value)} placeholder="example.com" /></Field>
              </>
            ) : null}
            {transport === "xhttp" && <Field label="XHTTP 模式"><select value={stringValue(networkSettings.mode, "auto")} onChange={(event) => updateNetwork("mode", event.target.value)}><option value="auto">自动</option><option value="packet-up">Packet Up</option><option value="stream-up">Stream Up</option><option value="stream-one">Stream One</option></select></Field>}
          </div>
        </div>
      )}

      {protocol === "vless" && (
        <div className="config-group">
          <div className="config-group__heading"><h3>VLESS 参数</h3><p>控制流控和解密方式。</p></div>
          <div className="form-grid form-grid--three config-fields">
            <Field label="流控"><select value={stringValue(settings.flow)} onChange={(event) => update(setValue(settings, "flow", event.target.value))}><option value="">不启用</option><option value="xtls-rprx-vision">XTLS Vision</option></select></Field>
            <Field label="解密方式"><select value={stringValue(settings.decryption, "none")} onChange={(event) => update(setValue(settings, "decryption", event.target.value))}><option value="none">none</option></select></Field>
          </div>
        </div>
      )}

      {protocol === "shadowsocks" && (
        <div className="config-group">
          <div className="config-group__heading"><h3>Shadowsocks 参数</h3><p>选择服务端与订阅客户端共同使用的加密方式。</p></div>
          <div className="form-grid form-grid--three config-fields">
            <Field label="加密方式" required><select value={cipher} onChange={(event) => updateCipher(event.target.value)}><option value="aes-128-gcm">aes-128-gcm</option><option value="aes-256-gcm">aes-256-gcm</option><option value="chacha20-ietf-poly1305">chacha20-ietf-poly1305</option><option value="2022-blake3-aes-128-gcm">2022-blake3-aes-128-gcm</option><option value="2022-blake3-aes-256-gcm">2022-blake3-aes-256-gcm</option></select></Field>
            {ss2022KeySize && <Field label="服务端密钥" required helper={`标准 Base64 编码，解码后必须为 ${ss2022KeySize} 字节`}><div className="input-with-action"><input className="mono" type="text" autoComplete="off" spellCheck={false} value={stringValue(settings.server_key)} onChange={(event) => update(setValue(settings, "server_key", event.target.value.trim()))} /><Button type="button" onClick={() => update(setValue(settings, "server_key", randomSS2022ServerKey(cipher)))}><RefreshCw size={15} aria-hidden="true" />随机生成</Button></div></Field>}
          </div>
        </div>
      )}

      {protocol === "hysteria" && (
        <div className="config-group">
          <div className="config-group__heading"><h3>Hysteria 参数</h3><p>Xray 仅支持 Hysteria 2，sing-box 可运行两个版本。</p></div>
          <div className="form-grid form-grid--three config-fields">
            <Field label="协议版本"><select value={String(numberValue(settings.version, 2))} onChange={(event) => update(setValue(settings, "version", Number(event.target.value)))}><option value="2">Hysteria 2</option>{kernel === "singbox" && <option value="1">Hysteria 1</option>}</select></Field>
            {numberValue(settings.version, 2) === 1 && <><Field label="上行带宽 Mbps"><input type="number" min="0" value={numberValue(settings.up_mbps)} onChange={(event) => update(setValue(settings, "up_mbps", Number(event.target.value)))} /></Field><Field label="下行带宽 Mbps"><input type="number" min="0" value={numberValue(settings.down_mbps)} onChange={(event) => update(setValue(settings, "down_mbps", Number(event.target.value)))} /></Field></>}
            <Field label="混淆方式"><select value={stringValue(settings.obfs)} onChange={(event) => update(setValue(settings, "obfs", event.target.value))}><option value="">不启用</option><option value="salamander">Salamander</option></select></Field>
            {stringValue(settings.obfs) && <Field label="混淆密码"><input type="password" autoComplete="new-password" value={stringValue(settings.obfs_password, stringValue(settings["obfs-password"]))} onChange={(event) => update(setValue(settings, "obfs_password", event.target.value))} /></Field>}
          </div>
        </div>
      )}

      {protocol === "tuic" && <div className="config-group"><div className="config-group__heading"><h3>TUIC 参数</h3><p>选择 QUIC 拥塞控制算法。</p></div><div className="form-grid form-grid--three config-fields"><Field label="拥塞控制"><select value={stringValue(settings.congestion_control, "bbr")} onChange={(event) => update(setValue(settings, "congestion_control", event.target.value))}><option value="bbr">BBR</option><option value="cubic">CUBIC</option><option value="new_reno">New Reno</option></select></Field></div></div>}

      {protocol === "anytls" && <div className="config-group"><div className="config-group__heading"><h3>AnyTLS 参数</h3><p>配置 TLS 记录填充规则。</p></div><div className="config-fields config-fields--single"><Field label="填充方案" helper="每行一条规则"><textarea rows={4} value={stringValue(settings.padding_scheme)} onChange={(event) => update(setValue(settings, "padding_scheme", event.target.value))} placeholder={"stop=8\n0=30-30\n1=100-400"} /></Field></div></div>}

      {protocol === "mieru" && <div className="config-group"><div className="config-group__heading"><h3>Mieru 参数</h3><p>选择底层传输与流量模式。</p></div><div className="form-grid form-grid--three config-fields"><Field label="底层传输"><select value={stringValue(settings.transport, "tcp")} onChange={(event) => update(setValue(settings, "transport", event.target.value))}><option value="tcp">TCP</option><option value="udp">UDP</option></select></Field><Field label="流量模式"><input value={stringValue(settings.traffic_pattern)} onChange={(event) => update(setValue(settings, "traffic_pattern", event.target.value))} placeholder="留空使用默认值" /></Field></div></div>}

      {tlsProtocols.has(protocol) && (
        <div className="config-group">
          <div className="config-group__heading"><h3>传输安全</h3><p>{requiredTLSProtocols.has(protocol) ? "当前协议必须配置 TLS 和证书。" : "按部署方式选择关闭、TLS 或 Reality。"}</p></div>
          <div className="segmented-control" role="group" aria-label="传输安全模式">
            {!requiredTLSProtocols.has(protocol) && <button type="button" className={currentTLSMode === "off" ? "active" : ""} onClick={() => update(withTLSMode(settings, "off"))}>关闭</button>}
            <button type="button" className={currentTLSMode === "tls" ? "active" : ""} onClick={() => update(withTLSMode(settings, "tls"))}>TLS</button>
            {realityProtocols.has(protocol) && <button type="button" className={currentTLSMode === "reality" ? "active" : ""} onClick={() => update(withTLSMode(settings, "reality"))}>Reality</button>}
          </div>
          {currentTLSMode === "reality" && (
            <div className="form-grid config-fields">
              <Field label="Reality 私钥" required error={credentialError} helper="随机生成会同时更新公钥和 Short ID"><div className="input-with-action"><input className="mono" type="text" autoComplete="off" spellCheck={false} value={stringValue(tls.private_key)} onChange={(event) => updateTLS("private_key", event.target.value.trim())} /><Button type="button" loading={realityGenerating} onClick={() => void generateRealityCredentials()}><RefreshCw size={15} aria-hidden="true" />随机生成</Button></div></Field>
              <Field label="Reality 公钥" required helper="用于 Clash Meta 客户端订阅"><input className="mono" spellCheck={false} value={stringValue(tls.public_key)} onChange={(event) => updateTLS("public_key", event.target.value.trim())} /></Field>
              <Field label="Short ID" required><input className="mono" value={stringValue(tls.short_id)} onChange={(event) => updateTLS("short_id", event.target.value)} placeholder="例如：6ba85179e30d4fc2" /></Field>
              <Field label="伪装域名" required><input value={stringValue(tls.server_name)} onChange={(event) => updateTLS("server_name", event.target.value)} placeholder="www.example.com" /></Field>
              <Field label="目标地址" required><input value={stringValue(tls.dest)} onChange={(event) => updateTLS("dest", event.target.value)} placeholder="www.example.com:443" /></Field>
              <Field label="客户端伪装指纹" required helper="随订阅下发给 Clash Meta 客户端"><select value={stringValue(tls.fingerprint, "chrome")} onChange={(event) => updateTLS("fingerprint", event.target.value)}>{realityFingerprints.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select></Field>
            </div>
          )}
          {currentTLSMode === "tls" && (
            <div className="form-grid config-fields">
              <Field label="服务器名称"><input value={stringValue(tls.server_name)} onChange={(event) => updateTLS("server_name", event.target.value)} placeholder="example.com" /></Field>
              <Field label="证书方式"><select value={certMode} onChange={(event) => updateCert("cert_mode", event.target.value)}><option value="none">使用 Corade 本机或上游 TLS</option><option value="self">自动生成自签证书</option><option value="file">读取服务器证书文件</option><option value="http">ACME HTTP-01</option><option value="dns">ACME DNS-01</option><option value="content">粘贴证书内容</option></select></Field>
              {["http", "dns"].includes(certMode) && <><Field label="证书域名" required><input value={stringValue(cert.domain)} onChange={(event) => updateCert("domain", event.target.value)} placeholder="node.example.com" /></Field><Field label="ACME 邮箱"><input type="email" value={stringValue(cert.email)} onChange={(event) => updateCert("email", event.target.value)} /></Field></>}
              {certMode === "http" && <Field label="HTTP-01 端口"><input type="number" min="1" max="65535" value={numberValue(cert.http_port, 80)} onChange={(event) => updateCert("http_port", Number(event.target.value))} /></Field>}
              {certMode === "dns" && <Field label="DNS Provider" helper="凭据使用系统设置中的 DNS 配置"><input value={stringValue(cert.dns_provider)} onChange={(event) => updateCert("dns_provider", event.target.value)} placeholder="cloudflare" /></Field>}
              {certMode === "file" && <><Field label="证书文件" required><input className="mono" value={stringValue(cert.cert_file)} onChange={(event) => updateCert("cert_file", event.target.value)} placeholder="/etc/ssl/node.crt" /></Field><Field label="私钥文件" required><input className="mono" value={stringValue(cert.key_file)} onChange={(event) => updateCert("key_file", event.target.value)} placeholder="/etc/ssl/node.key" /></Field></>}
              {certMode === "content" && <><Field label="证书内容" required><textarea rows={6} className="mono" value={stringValue(cert.cert_content)} onChange={(event) => updateCert("cert_content", event.target.value)} /></Field><Field label="私钥内容" required><textarea rows={6} className="mono" value={stringValue(cert.key_content)} onChange={(event) => updateCert("key_content", event.target.value)} /></Field></>}
            </div>
          )}
        </div>
      )}

      <div className="config-group config-group--last">
        <div className="config-group__heading"><h3>连接选项</h3><p>仅在前置负载均衡或高并发场景中启用。</p></div>
        <div className="option-toggles">
          <label><input type="checkbox" checked={booleanValue(settings.accept_proxy_protocol)} onChange={(event) => update(setValue(settings, "accept_proxy_protocol", event.target.checked))} /><span><strong>接受 Proxy Protocol</strong><small>从负载均衡器获取真实来源地址</small></span></label>
          {multiplexProtocols.has(protocol) && <label><input type="checkbox" checked={booleanValue(multiplex.enabled)} onChange={(event) => update(setObjectValue(settings, "multiplex", "enabled", event.target.checked))} /><span><strong>启用多路复用</strong><small>在单个连接中承载多个数据流</small></span></label>}
        </div>
        {multiplexProtocols.has(protocol) && booleanValue(multiplex.enabled) && <div className="form-grid form-grid--three config-fields"><Field label="复用协议"><select value={stringValue(multiplex.protocol, "h2mux")} onChange={(event) => update(setObjectValue(settings, "multiplex", "protocol", event.target.value))}><option value="h2mux">h2mux</option><option value="smux">smux</option><option value="yamux">yamux</option></select></Field><Field label="最大连接数"><input type="number" min="0" value={numberValue(multiplex.max_connections, 4)} onChange={(event) => update(setObjectValue(settings, "multiplex", "max_connections", Number(event.target.value)))} /></Field></div>}
      </div>
    </div>
  );
}
