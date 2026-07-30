export type StatusTone = "success" | "warning" | "danger" | "info" | "neutral";

export interface Admin {
  id: string;
  email: string;
  name: string;
  status: string;
}

export interface Session {
  admin: Admin;
  csrf_token: string;
  expires_at?: string;
}

export interface Machine {
  id: string;
  name: string;
  region: string;
  host: string;
  labels: Record<string, string>;
  notes?: string;
  status: string;
  agent_version: string;
  latest_agent_version: string;
  kernel_type: string;
  last_heartbeat_at: string | null;
  metrics: {
    cpu?: number;
    mem?: { total?: number; used?: number };
    disk?: { total?: number; used?: number };
    [key: string]: unknown;
  };
  metrics_sampled_at: string | null;
  agent_upgrade_task_id?: string;
  agent_upgrade_requested_at?: string | null;
  agent_upgrade_dispatched_at?: string | null;
  agent_protocol?: "legacy" | "v2";
  agent_v2_last_seen_at?: string | null;
  node_count: number;
  created_at: string;
  updated_at: string;
}

export interface Node {
  id: string;
  agent_id: number;
  machine_id: string;
  machine_name: string;
  route_policy_id: string | null;
  name: string;
  protocol: string;
  listen_ip: string;
  server_port: number;
  kernel_type: string;
  config: Record<string, unknown>;
  status: string;
  current_revision: number;
  applied_revision: number;
  last_report_at: string | null;
  last_error?: string;
  created_at: string;
  updated_at: string;
  endpoints: NodeEndpoint[];
}

export interface NodeEndpoint {
  id?: string;
  name: string;
  host: string;
  port: number;
  status: "active" | "disabled";
  sort_order: number;
}

export interface AccessGroup {
  id: string;
  name: string;
  status: string;
  notes?: string;
  node_count: number;
  user_count: number;
  node_ids: string[];
  created_at: string;
  updated_at: string;
}

export interface Plan {
  id: string;
  access_group_id: string;
  access_group_name: string;
  name: string;
  status: string;
  traffic_limit_bytes: number;
  speed_limit_mbps: number;
  device_limit: number;
  reset_strategy: string;
  default_valid_days: number;
  notes?: string;
  user_count: number;
  created_at: string;
  updated_at: string;
}

export interface User {
  id: string;
  agent_id: number;
  role: "admin" | "user" | "friend";
  plan_id: string | null;
  plan_name: string | null;
  name: string;
  email: string | null;
  uuid: string;
  subscription_token_prefix: string;
  subscription_available: boolean;
  status: string;
  traffic_used_bytes: number;
  expires_at: string | null;
  notes?: string;
  created_at: string;
  updated_at: string;
}

export interface RoutePolicy {
  id: string;
  name: string;
  status: string;
  current_revision: number;
  notes?: string;
  node_count: number;
  created_at: string;
  updated_at: string;
}

export interface Outbound {
  id: string;
  name: string;
  tag: string;
  protocol: string;
  settings: Record<string, unknown>;
  proxy_tag: string;
  kernel_support: string[];
  status: string;
  created_at: string;
  updated_at: string;
}

export interface Overview {
  machines_total: number;
  machines_online: number;
  machines_offline: number;
  nodes_total: number;
  nodes_published: number;
  admins_active: number;
  users_active: number;
  friends_active: number;
  traffic_today_bytes: number;
  traffic_today_upload_bytes: number;
  traffic_today_download_bytes: number;
  node_traffic_ranking: TrafficRank[];
  user_traffic_ranking: TrafficRank[];
}

export interface TrafficRank {
  id: string;
  name: string;
  upload_bytes: number;
  download_bytes: number;
  total_bytes: number;
}

export interface HistoricalData {
  retention: {
    devices_days: number;
    traffic_days: number;
  };
  traffic: DailyTraffic[];
  machine_metrics: MetricSample[];
  node_metrics: MetricSample[];
  devices: DeviceHistory[];
}

export interface DailyTraffic {
  day: string;
  upload_bytes: number;
  download_bytes: number;
}

export interface MetricSample {
  resource_id: string;
  resource_name: string;
  sampled_at: string;
  metrics: Record<string, unknown>;
}

export interface DeviceHistory {
  id: string;
  user_id: string;
  user_name: string;
  node_id: string;
  node_name: string;
  ip_address: string;
  first_seen_at: string;
  last_seen_at: string;
  online: boolean;
}

export interface Setting {
  key: string;
  value: unknown;
  sensitive: boolean;
  version: number;
  updated_at: string;
}

export interface ApiErrorBody {
  code: string;
  message: string;
  fields?: Record<string, string>;
}

export interface Envelope<T> {
  data: T;
  meta: { request_id: string };
  error: ApiErrorBody | null;
}
