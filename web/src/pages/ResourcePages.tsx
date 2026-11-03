import { useLocation, useParams } from "wouter";
import { KeyRound } from "lucide-react";
import { formatBytes, formatDateWithYear, formatPreciseDate } from "../components/ui";
import { defaultOutboundSettings } from "../components/OutboundConfigForm";
import { MachineMetrics } from "../components/MachineMetrics";
import { ResourceDate } from "../components/ResourceList";
import { useResource } from "../hooks";
import { agentUpgradeLabel, localDateTime } from "../resourceModel";
import type { Resource, ResourceDefinition } from "../resourceDefinitions";
import type { AccessGroup, Machine, Node, Outbound, Plan, RoutePolicy, User } from "../types";
import { ResourceListPage } from "./ResourceListPage";
import { ResourceEditorPage } from "./ResourceEditorPage";

function ResourcePage<T extends Resource>(definition: ResourceDefinition<T>) {
  const [path] = useLocation();
  const { id } = useParams();
  const editor = /\/new$|\/[^/]+\/edit$/.test(path);
  return editor ? <ResourceEditorPage key={definition.endpoint + ":" + (id || "new")} definition={definition} id={id} /> : <ResourceListPage {...definition} />;
}

const primary = (name: string, id: string, detail?: string) => <><strong className="cell-primary">{name}</strong><span className="resource-id">{detail || id}</span></>;

export function MachinesPage() {
  return <ResourcePage<Machine>
    title="服务器"
    description="管理安装 Corade Agent 的物理机或虚拟机"
    endpoint="/machines"
    list={{ kind: "machines", searchPlaceholder: "搜索名称、IP 或域名", note: "资源数据随心跳更新。离线服务器保留最后上报信息，请留意采样时间。", filter: { label: "按区域筛选服务器", allLabel: "全部区域", value: (item) => item.region || "未设置" } }}
    createLabel="添加服务器"
    installable
    upgradeable
    identityResettable
    refreshIntervalMs={15_000}
    enabledStatus="pending"
    defaults={{ name: "", region: "", host: "", labels: "{}", notes: "" }}
    fields={[{ key: "name", label: "服务器名称", required: true, placeholder: "例如：香港边缘 01" }, { key: "region", label: "区域", placeholder: "例如：香港" }, { key: "host", label: "IP 或域名", placeholder: "hk01.example.net" }, { key: "labels", label: "标签 JSON", type: "json", helper: "用于筛选和自动化识别", placeholder: "{}" }, { key: "notes", label: "备注", type: "textarea" }]}
    transform={(v) => ({ name: v.name, region: v.region, host: v.host, labels: JSON.parse(v.labels || "{}"), notes: v.notes })}
    toValues={(item) => ({ name: item.name, region: item.region, host: item.host, labels: JSON.stringify(item.labels ?? {}, null, 2), notes: item.notes ?? "" })}
    columns={[
      { label: "服务器", width: "25%", render: (item) => primary(item.name, item.id, item.host || item.id) },
      { label: "区域 / 节点", width: "12%", render: (item) => <>{item.region || "未设置"}<span className="resource-id">{item.node_count} 个节点</span></> },
      { label: "资源占用", width: "17%", render: (item) => <MachineMetrics machine={item} /> },
      { label: "Agent", width: "18%", render: (item) => <><span className="cell-primary mono">{item.agent_version || "未接入"}</span><span className={`resource-id ${item.agent_protocol === "v2" ? "text-success" : ""}`}>{item.agent_protocol === "v2" ? "V2 加密" : "旧通讯"} · {agentUpgradeLabel(item)}</span></> },
      { label: "最后心跳", render: (item) => <ResourceDate value={item.last_heartbeat_at} formatted={formatPreciseDate(item.last_heartbeat_at)} /> },
    ]}
  />;
}

function TrafficUsage({ used, limit }: { used: number; limit: number }) {
  const percent = limit > 0 ? Math.min(100, Math.max(0, used / limit * 100)) : 0;
  return <div className="traffic-usage">
    <span><strong>{formatBytes(used)}</strong><small>/ {limit > 0 ? formatBytes(limit) : "不限"}</small></span>
    {limit > 0 && <span className="traffic-usage__track" role="progressbar" aria-label="流量使用比例" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(percent)}><i style={{ width: `${percent}%` }} /></span>}
  </div>;
}

export function AccessGroupsPage() {
  const { data: nodes, loading: relatedLoading, error: relatedError, reload: retryRelated } = useResource<Node[]>("/nodes", []);
  return <ResourcePage<AccessGroup> related={{ loading: relatedLoading, error: relatedError, retry: () => { void retryRelated(); } }} list={{ kind: "groups", searchPlaceholder: "搜索权限组名称或备注", note: "权限组定义节点访问范围，不是后台管理员角色。节点需处于已发布状态才可使用。" }} title="权限组" description="控制用户和朋友可以访问的节点范围" endpoint="/access-groups" createLabel="添加权限组" defaults={{ name: "", notes: "", node_ids: "" }} fields={[{ key: "name", label: "权限组名称", required: true, placeholder: "例如：标准线路" }, { key: "node_ids", label: "可访问节点", type: "node-picker", options: nodes.map((item) => ({ value: item.id, label: item.name, detail: `${item.protocol.toUpperCase()} · ${item.machine_name} · 端口 ${item.server_port} · ${item.status === "published" ? "已发布" : item.status === "disabled" ? "已停用" : item.status === "draft" ? "草稿" : "异常"}`, keywords: `${item.id} ${item.machine_name} ${item.protocol} ${item.server_port}` })) }, { key: "notes", label: "备注", type: "textarea", placeholder: "选填，仅管理员可见" }]} transform={(v) => ({ name: v.name, notes: v.notes, node_ids: v.node_ids.split(",").filter(Boolean) })} toValues={(item) => ({ name: item.name, notes: item.notes ?? "", node_ids: (item.node_ids ?? []).join(",") })} columns={[{ label: "权限组", width: "25%", render: (item) => primary(item.name, item.id, item.notes) }, { label: "可访问节点", width: "31%", render: (item) => <><span className="mono">{item.node_count} 个节点</span><div className="resource-node-chips">{(item.node_ids ?? []).slice(0, 3).map((id) => <span key={id}>{nodes.find((node) => node.id === id)?.name || id}</span>)}{(item.node_ids?.length ?? 0) > 3 && <span>+{item.node_ids.length - 3}</span>}</div></> }, { label: "账号数", width: "9%", render: (item) => <span className="mono">{item.user_count}</span> }, { label: "更新时间", render: (item) => <ResourceDate value={item.updated_at} formatted={formatDateWithYear(item.updated_at)} /> }]} />;
}

export function PlansPage() {
  const { data: groups, loading: relatedLoading, error: relatedError, reload: retryRelated } = useResource<AccessGroup[]>("/access-groups", []);
  return <ResourcePage<Plan> related={{ loading: relatedLoading, error: relatedError, retry: () => { void retryRelated(); } }}
    title="套餐"
    description="定义流量、重置周期、速率、设备数和默认访问范围"
    endpoint="/plans"
    list={{ kind: "plans", searchPlaceholder: "搜索套餐名称或权限组", note: "套餐由管理员手动分配。自然月按站点时区每月 1 日重置流量，不自动重置则持续累计。" }}
    createLabel="添加套餐"
    defaults={{ name: "", access_group_id: groups[0]?.id ?? "", traffic_gb: "500", speed_limit_mbps: "0", device_limit: "3", reset_strategy: "calendar_month", default_valid_days: "30", notes: "" }}
    fields={[
      { key: "name", label: "套餐名称", required: true },
      { key: "access_group_id", label: "默认权限组", type: "select", required: true, options: [{ value: "", label: "请选择权限组" }, ...groups.map((item) => ({ value: item.id, label: item.name }))] },
      { key: "traffic_gb", label: "套餐流量（GB）", type: "number", required: true },
      { key: "reset_strategy", label: "流量重置", type: "select", required: true, helper: "自然月按站点时区每月 1 日 00:00 归零；不自动重置会从当前用量继续累计", options: [{ value: "calendar_month", label: "自然月重置" }, { value: "never", label: "不自动重置（持续累计）" }] },
      { key: "speed_limit_mbps", label: "速率限制（Mbps）", type: "number" },
      { key: "device_limit", label: "同时在线设备数", type: "number" },
      { key: "default_valid_days", label: "默认有效天数", type: "number" },
      { key: "notes", label: "备注", type: "textarea" },
    ]}
    transform={(v) => ({ name: v.name, access_group_id: v.access_group_id, traffic_limit_bytes: Number(v.traffic_gb) * 1024 ** 3, speed_limit_mbps: Number(v.speed_limit_mbps || 0), device_limit: Number(v.device_limit || 0), reset_strategy: v.reset_strategy, default_valid_days: Number(v.default_valid_days || 0), notes: v.notes })}
    toValues={(item) => ({ name: item.name, access_group_id: item.access_group_id, traffic_gb: String(item.traffic_limit_bytes / 1024 ** 3), speed_limit_mbps: String(item.speed_limit_mbps), device_limit: String(item.device_limit), reset_strategy: item.reset_strategy, default_valid_days: String(item.default_valid_days), notes: item.notes ?? "" })}
    columns={[
      { label: "套餐 / 权限组", width: "25%", render: (item) => primary(item.name, item.id, item.access_group_name) },
      { label: "流量", width: "13%", className: "mono", render: (item) => formatBytes(item.traffic_limit_bytes) },
      { label: "速率", width: "13%", className: "mono", render: (item) => `${item.speed_limit_mbps || "不限"}${item.speed_limit_mbps ? " Mbps" : ""}` },
      { label: "设备", width: "8%", render: (item) => item.device_limit ? `${item.device_limit} 台` : "不限" },
      { label: "账号数", width: "9%", render: (item) => <span className="mono">{item.user_count}</span> },
      { label: "重置周期", render: (item) => <>{item.reset_strategy === "never" ? "不自动重置" : "自然月"}<span className="resource-id">{item.default_valid_days ? `${item.default_valid_days} 天有效期` : "不限制有效期"}</span></> },
    ]}
  />;
}

export function UsersPage() {
  const { data: plans, loading: relatedLoading, error: relatedError, reload: retryRelated } = useResource<Plan[]>("/plans", []);
  return <ResourcePage<User> related={{ loading: relatedLoading, error: relatedError, retry: () => { void retryRelated(); } }}
    title="订阅账号"
    description="管理员可登录后台并使用订阅；用户与朋友仅能使用订阅"
    endpoint="/users"
    list={{ kind: "users", searchPlaceholder: "搜索名称、邮箱或门户账号", note: "管理员可登录后台，用户与朋友使用订阅中心。管理员代入订阅中心时保持只读。", categories: [{ value: "user", label: "用户", matches: (item) => item.role === "user" }, { value: "friend", label: "朋友", matches: (item) => item.role === "friend" }, { value: "admin", label: "管理员", matches: (item) => item.role === "admin" }], filter: { label: "按套餐筛选订阅账号", allLabel: "全部套餐", value: (item) => item.plan_name || "未分配" } }}
    createLabel="添加用户或朋友"
    subscriptionActions
	portalActions
    quickAccountCreation
    deletable
    disabledStatus="paused"
    protectedItem={(item) => item.role === "admin"}
    defaults={{ role: "user", name: "", email: "", plan_id: plans[0]?.id ?? "", expires_at: "", notes: "", portal_login: "", portal_password: "" }}
    fields={[
      { key: "role", label: "账号分类", type: "select", lockedValue: "admin", options: [{ value: "admin", label: "管理员", exclusive: true }, { value: "user", label: "用户" }, { value: "friend", label: "朋友" }] },
      { key: "uuid", label: "UDID", editOnly: true, required: true, helper: "修改后会随节点成员配置同步到 Corade" },
      { key: "name", label: "账号名称", required: true },
      { key: "email", label: "邮箱", type: "email" },
      { key: "plan_id", label: "套餐", type: "select", options: [{ value: "", label: "暂不分配" }, ...plans.map((item) => ({ value: item.id, label: item.name }))] },
      { key: "expires_at", label: "到期时间", type: "datetime-local" },
      { key: "notes", label: "内部备注", type: "textarea" },
		{ key: "portal_login", label: "门户登录账号", helper: "用户在 /edu 使用的唯一账号。已有邮箱会自动预填。", disabled: (item) => item?.role === "admin" },
		{ key: "portal_password", label: "设置或重置门户密码", type: "password", helper: "至少 8 位；编辑时留空则不修改。管理员订阅账号使用后台一键进入。", disabled: (item) => item?.role === "admin" },
    ]}
    transform={(v, editing) => {
      const email = v.email.trim();
      const values = { role: v.role, name: v.name.trim(), email: editing ? email : email || null, plan_id: editing ? v.plan_id : v.plan_id || null, expires_at: v.expires_at ? new Date(v.expires_at).toISOString() : editing ? "" : null, notes: v.notes, portal_login: v.portal_login.trim(), portal_password: v.portal_password };
      return editing ? { ...values, uuid: v.uuid } : values;
    }}
    toValues={(item) => ({ role: item.role, uuid: item.uuid, name: item.name, email: item.email ?? "", plan_id: item.plan_id ?? "", expires_at: localDateTime(item.expires_at), notes: item.notes ?? "", portal_login: item.portal_login ?? "", portal_password: "" })}
    columns={[
      { label: "账号", width: "21%", render: (item) => primary(item.name, item.id, item.email || item.id) },
      { label: "分类", width: "8%", render: (item) => <span className="resource-role">{item.role === "admin" && <KeyRound size={12} aria-hidden="true" />}{item.role === "admin" ? "管理员" : item.role === "friend" ? "朋友" : "用户"}</span> },
      { label: "套餐", width: "11%", render: (item) => item.plan_name || "未分配" },
      { label: "流量用量", width: "16%", render: (item) => <TrafficUsage used={item.traffic_used_bytes} limit={item.traffic_limit_bytes} /> },
		{ label: "门户", width: "13%", render: (item) => item.role === "admin" ? <span>后台进入<span className="resource-id">管理员只读代入</span></span> : item.portal_enabled ? item.portal_login : "未启用" },
      { label: "到期", render: (item) => item.expires_at ? <ResourceDate value={item.expires_at} formatted={formatDateWithYear(item.expires_at)} /> : "长期" },
    ]}
  />;
}

export function RoutesPage() {
  const { data: outbounds, loading: relatedLoading, error: relatedError, reload: retryRelated } = useResource<Outbound[]>("/outbounds", []);
  const initialRules = JSON.stringify([]);
  const defaultOutboundOptions = [{ value: "", label: "默认直连（仅按下方规则处理）" }, ...outbounds.filter((item) => item.status === "active" && !["direct", "block"].includes(item.protocol) && item.kernel_support.includes("xray")).map((item) => ({ value: item.tag, label: `${item.name} · ${item.tag}` }))];
  return <ResourcePage<RoutePolicy> related={{ loading: relatedLoading, error: relatedError, retry: () => { void retryRelated(); } }} title="路由策略" description="按顺序匹配域名、IP、端口与网络协议，并执行直连、阻断或指定出站" endpoint="/route-policies" list={{ kind: "routes", searchPlaceholder: "搜索策略名称、备注或 ID", note: "节点分别绑定默认、管理员、用户与朋友策略；未单独绑定时回退默认策略。规则从上到下匹配。", categories: [{ value: "default", label: "默认", matches: (item) => !item.scope || item.scope === "default" }, { value: "admin", label: "管理员", matches: (item) => item.scope === "admin" }, { value: "member", label: "用户与朋友", matches: (item) => item.scope === "member" }] }} createLabel="添加路由策略" defaults={{ name: "", scope: "default", notes: "", default_outbound_tag: "", rules: initialRules }} fields={[{ key: "name", label: "策略名称", required: true, placeholder: "例如：流媒体直连" }, { key: "scope", label: "适用范围", type: "select", options: [{ value: "default", label: "默认策略" }, { value: "admin", label: "管理员订阅" }, { value: "member", label: "用户与朋友订阅" }], helper: "节点可分别绑定管理员策略和用户/朋友策略；未绑定时回退默认策略。" }, { key: "default_outbound_tag", label: "默认流量出口", type: "select", options: defaultOutboundOptions, helper: "选择出口后，未被规则命中的流量将通过该出口；未选择时保持直连。" }, { key: "notes", label: "策略备注", type: "textarea", placeholder: "选填，例如适用区域或变更原因" }, { key: "rules", label: "规则编排", type: "route-rules", routeOutbounds: outbounds }]} transform={(v) => ({ name: v.name.trim(), scope: v.scope, notes: v.notes.trim(), default_outbound_tag: v.default_outbound_tag, rules: JSON.parse(v.rules || "[]") })} toValues={(item) => ({ name: item.name, scope: item.scope ?? "default", notes: item.notes ?? "", default_outbound_tag: item.default_outbound_tag ?? "", rules: JSON.stringify(item.rules ?? []) })} columns={[{ label: "策略", render: (item) => primary(item.name, item.id, item.notes) }, { label: "适用范围", render: (item) => item.scope === "admin" ? "管理员" : item.scope === "member" ? "用户与朋友" : "默认" }, { label: "默认出口", render: (item) => item.default_outbound_tag || "直连" }, { label: "规则", render: (item) => <span className="mono">{item.rules?.length ?? 0}</span> }, { label: "当前版本", render: (item) => <span className="mono">rev {item.current_revision}</span> }, { label: "绑定节点", render: (item) => <span className="mono">{item.node_count}</span> }, { label: "更新时间", render: (item) => <ResourceDate value={item.updated_at} formatted={formatDateWithYear(item.updated_at)} /> }]} />;
}

export function OutboundsPage() {
  const { data: outbounds, loading: relatedLoading, error: relatedError, reload: retryRelated } = useResource<Outbound[]>("/outbounds", []);
  return <ResourcePage<Outbound> related={{ loading: relatedLoading, error: relatedError, retry: () => { void retryRelated(); } }} title="出站" description="维护路由策略可选择的 Xray 代理出站" endpoint="/outbounds" list={{ kind: "outbounds", searchPlaceholder: "搜索名称、标记或服务器地址", note: "统一维护可供路由选择的 Xray 出站。上游用于链式代理；当前不提供出站连通性探测。", categories: ["socks", "vless", "vmess", "trojan", "shadowsocks", "http", "wireguard"].map((protocol) => ({ value: protocol, label: protocol === "shadowsocks" ? "SS" : protocol.toUpperCase(), matches: (item) => item.protocol === protocol })) }} createLabel="添加出站" defaults={{ name: "", tag: "", protocol: "socks", proxy_tag: "", settings: JSON.stringify(defaultOutboundSettings("socks")), kernel_support: "xray" }} fields={[{ key: "name", label: "出站名称", required: true, placeholder: "例如：新加坡 SOCKS 出站" }, { key: "tag", label: "唯一标记", required: true, placeholder: "sg-socks" }, { key: "settings", label: "协议与连接参数", type: "outbound-config", required: true, routeOutbounds: outbounds }]} transform={(v) => ({ name: v.name.trim(), tag: v.tag.trim(), protocol: v.protocol, proxy_tag: v.proxy_tag, settings: JSON.parse(v.settings || "{}"), kernel_support: ["xray"] })} toValues={(item) => ({ name: item.name, tag: item.tag, protocol: item.protocol, proxy_tag: item.proxy_tag, settings: JSON.stringify(item.settings ?? {}), kernel_support: (item.kernel_support ?? ["xray"]).join(",") })} columns={[{ label: "出站", render: (item) => primary(item.name, item.id, item.tag) }, { label: "协议", render: (item) => <span className="protocol-label">{item.protocol}</span> }, { label: "服务器", render: (item) => <span className="mono">{outboundAddress(item.settings)}</span> }, { label: "上游", render: (item) => <>{item.proxy_tag || "无"}<span className="resource-id">{item.proxy_tag ? "通过上游出站连接" : "直接连接目标"}</span></> }, { label: "内核", render: (item) => item.kernel_support.map((kernel) => kernel === "xray" ? "Xray" : kernel).join(" / ") }, { label: "更新时间", render: (item) => <ResourceDate value={item.updated_at} formatted={formatDateWithYear(item.updated_at)} /> }]} />;
}

function outboundAddress(settings: Record<string, unknown>) {
  const servers = settings.servers ?? settings.vnext;
  const server = Array.isArray(servers) ? servers[0] as Record<string, unknown> | undefined : undefined;
  if (server?.address) return `${server.address}:${server.port ?? "—"}`;
  const peers = settings.peers;
  const peer = Array.isArray(peers) ? peers[0] as Record<string, unknown> | undefined : undefined;
  return String(peer?.endpoint || "—");
}
