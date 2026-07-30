import type {
  AccessGroup,
  Machine,
  Node,
  Outbound,
  Overview,
  Plan,
  RoutePolicy,
  Session,
  Setting,
  User,
} from "./types";

const now = new Date().toISOString();

export const demoSession: Session = {
  admin: { id: "adm_demo", email: "admin@cpanel.local", name: "管理员", status: "active" },
  csrf_token: "csrf_demo",
};

export const demoMachines: Machine[] = [
  { id: "mch_hk01", name: "香港边缘 01", region: "香港", host: "hk01.example.net", labels: { tier: "edge" }, status: "online", agent_version: "0.8.4", kernel_type: "singbox", last_heartbeat_at: now, node_count: 3, created_at: now, updated_at: now },
  { id: "mch_sg02", name: "新加坡边缘 02", region: "新加坡", host: "sg02.example.net", labels: { tier: "edge" }, status: "online", agent_version: "0.8.4", kernel_type: "singbox", last_heartbeat_at: now, node_count: 2, created_at: now, updated_at: now },
  { id: "mch_jp01", name: "日本边缘 01", region: "日本", host: "jp01.example.net", labels: {}, status: "offline", agent_version: "0.8.2", kernel_type: "xray", last_heartbeat_at: "2026-07-29T05:18:00Z", node_count: 1, created_at: now, updated_at: now },
];

export const demoRoutes: RoutePolicy[] = [
  { id: "rte_asia", name: "亚洲流媒体分流", status: "published", current_revision: 12, node_count: 4, created_at: now, updated_at: now },
  { id: "rte_direct", name: "默认直连", status: "published", current_revision: 3, node_count: 2, created_at: now, updated_at: now },
];

export const demoNodes: Node[] = [
  { id: "nod_1042", agent_id: 1042, machine_id: "mch_hk01", machine_name: "香港边缘 01", route_policy_id: "rte_asia", name: "香港 VLESS 主入口", protocol: "vless", listen_ip: "0.0.0.0", server_port: 443, kernel_type: "singbox", config: { transport: "tcp", tls: { enabled: true, mode: "reality" } }, endpoints: [{ name: "香港 VLESS 主入口", host: "hk01.example.net", port: 443, status: "active", sort_order: 0 }], status: "published", current_revision: 31, applied_revision: 31, last_report_at: now, created_at: now, updated_at: now },
  { id: "nod_1046", agent_id: 1046, machine_id: "mch_hk01", machine_name: "香港边缘 01", route_policy_id: "rte_direct", name: "香港 Hysteria2", protocol: "hysteria2", listen_ip: "0.0.0.0", server_port: 8443, kernel_type: "singbox", config: { up_mbps: 1000, down_mbps: 1000 }, endpoints: [], status: "draft", current_revision: 18, applied_revision: 17, last_report_at: now, created_at: now, updated_at: now },
  { id: "nod_1088", agent_id: 1088, machine_id: "mch_sg02", machine_name: "新加坡边缘 02", route_policy_id: "rte_asia", name: "新加坡 AnyTLS", protocol: "anytls", listen_ip: "0.0.0.0", server_port: 443, kernel_type: "singbox", config: { padding_scheme: [] }, endpoints: [], status: "published", current_revision: 9, applied_revision: 9, last_report_at: now, created_at: now, updated_at: now },
  { id: "nod_1112", agent_id: 1112, machine_id: "mch_jp01", machine_name: "日本边缘 01", route_policy_id: null, name: "日本 Trojan 备用", protocol: "trojan", listen_ip: "0.0.0.0", server_port: 9443, kernel_type: "xray", config: { transport: "tcp", tls: { enabled: true } }, endpoints: [], status: "error", current_revision: 7, applied_revision: 6, last_report_at: "2026-07-29T05:18:00Z", last_error: "Agent 离线，配置尚未应用", created_at: now, updated_at: now },
];

export const demoGroups: AccessGroup[] = [
  { id: "grp_standard", name: "标准线路", status: "active", node_count: 3, user_count: 842, node_ids: ["nod_1042", "nod_1046", "nod_1088"], created_at: now, updated_at: now },
  { id: "grp_fast", name: "高速线路", status: "active", node_count: 4, user_count: 462, node_ids: ["nod_1042", "nod_1046", "nod_1088", "nod_1112"], created_at: now, updated_at: now },
];

export const demoPlans: Plan[] = [
  { id: "pln_standard", access_group_id: "grp_standard", access_group_name: "标准线路", name: "标准套餐", status: "active", traffic_limit_bytes: 536870912000, speed_limit_mbps: 100, device_limit: 3, reset_strategy: "calendar_month", default_valid_days: 30, user_count: 611, created_at: now, updated_at: now },
  { id: "pln_friend", access_group_id: "grp_fast", access_group_name: "高速线路", name: "朋友共享", status: "active", traffic_limit_bytes: 1099511627776, speed_limit_mbps: 300, device_limit: 5, reset_strategy: "calendar_month", default_valid_days: 365, user_count: 28, created_at: now, updated_at: now },
];

export const demoUsers: User[] = [
  { id: "usr_1001", agent_id: 1001, role: "user", plan_id: "pln_standard", plan_name: "标准套餐", name: "li.ming", email: "li.ming@example.com", uuid: "45f62d12-5322-48e8-81df-16e9523b88f4", subscription_token_prefix: "cps_v9k3", subscription_available: true, status: "active", traffic_used_bytes: 12884901888, expires_at: "2026-12-31T16:00:00Z", created_at: now, updated_at: now },
  { id: "usr_1002", agent_id: 1002, role: "friend", plan_id: "pln_friend", plan_name: "朋友共享", name: "chen", email: null, uuid: "4da0948c-b38f-4685-a055-57cb67da55c5", subscription_token_prefix: "cps_m2c8", subscription_available: true, status: "active", traffic_used_bytes: 4294967296, expires_at: null, created_at: now, updated_at: now },
];

export const demoOutbounds: Outbound[] = [
  { id: "out_direct", name: "默认直连", tag: "direct", protocol: "direct", settings: {}, proxy_tag: "", kernel_support: ["singbox", "xray"], status: "active", created_at: now, updated_at: now },
  { id: "out_sg", name: "新加坡代理", tag: "sg-proxy", protocol: "vless", settings: { server: "10.4.2.12", port: 443 }, proxy_tag: "", kernel_support: ["singbox", "xray"], status: "active", created_at: now, updated_at: now },
];

export const demoOverview: Overview = { machines_total: 3, machines_online: 2, machines_offline: 1, nodes_total: 4, nodes_published: 2, admins_active: 1, users_active: 1284, friends_active: 28, traffic_today_bytes: 7237010223104, traffic_today_upload_bytes: 1546188226560, traffic_today_download_bytes: 5690821996544, node_traffic_ranking: [{ id: "nod_1042", name: "香港 VLESS 主入口", upload_bytes: 824633720832, download_bytes: 2748779069440, total_bytes: 3573412790272 }], user_traffic_ranking: [{ id: "usr_1001", name: "li.ming", upload_bytes: 5368709120, download_bytes: 7516192768, total_bytes: 12884901888 }] };

export const demoSettings: Record<string, Setting[]> = {
  site: [
    { key: "platform_name", value: "CPanel", sensitive: false, version: 1, updated_at: now },
    { key: "site_url", value: "https://panel.example.com", sensitive: false, version: 1, updated_at: now },
    { key: "timezone", value: "Asia/Shanghai", sensitive: false, version: 1, updated_at: now },
  ],
  agent: [
    { key: "heartbeat_seconds", value: 60, sensitive: false, version: 1, updated_at: now },
    { key: "offline_threshold_seconds", value: 180, sensitive: false, version: 1, updated_at: now },
  ],
  security: [], node_defaults: [], certificate: [], subscription: [], retention: [],
};

export const demoAuditEvents = [
  { id: "aud_01", admin_id: "adm_demo", admin_name: "管理员", action: "node.publish", resource_type: "node", resource_id: "nod_1042", changes: { revision: 31 }, ip_address: "127.0.0.1", request_id: "req_demo_01", created_at: now },
  { id: "aud_02", admin_id: "adm_demo", admin_name: "管理员", action: "node.update", resource_type: "node", resource_id: "nod_1046", changes: { server_port: 8443 }, ip_address: "127.0.0.1", request_id: "req_demo_02", created_at: "2026-07-29T06:42:00Z" },
  { id: "aud_03", admin_id: "adm_demo", admin_name: "管理员", action: "admin.login", resource_type: "admin", resource_id: "adm_demo", changes: {}, ip_address: "127.0.0.1", request_id: "req_demo_03", created_at: "2026-07-29T06:31:00Z" },
];

export function demoResource(path: string): unknown {
  if (path === "/session") return demoSession;
  if (path === "/overview") return demoOverview;
  if (path === "/machines") return demoMachines;
  if (path === "/nodes") return demoNodes;
  if (path.startsWith("/nodes/")) return demoNodes.find((item) => item.id === path.split("/")[2]);
  if (path === "/access-groups") return demoGroups;
  if (path === "/plans") return demoPlans;
  if (path === "/users") return demoUsers;
  if (path === "/route-policies") return demoRoutes;
  if (path === "/outbounds") return demoOutbounds;
  if (path.startsWith("/settings/")) return demoSettings[path.split("/")[2]] ?? [];
  if (path === "/audit-events") return demoAuditEvents;
  return null;
}
