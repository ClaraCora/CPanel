import type { Node } from "./types";

type DemoRevision = {
  revision: number;
  created_at: string;
  published_by: string;
  restorable: boolean;
  config: Node["config"];
  snapshot?: { node: ReturnType<typeof document>; spec: { settings: Node["config"] } };
};

const histories = new Map<string, DemoRevision[]>();
function document(node: Node) {
  return structuredClone({ name: node.name, machine_id: node.machine_id, protocol: node.protocol, listen_ip: node.listen_ip, server_port: node.server_port, kernel_type: node.kernel_type, config: node.config, endpoints: node.endpoints, route_policy_id: node.route_policy_id, admin_route_policy_id: node.admin_route_policy_id, member_route_policy_id: node.member_route_policy_id });
}

function snapshot(node: Node): DemoRevision {
  return { revision: node.current_revision, created_at: node.updated_at, published_by: "adm_demo", restorable: true, config: structuredClone(node.config), snapshot: { node: document(node), spec: { settings: structuredClone(node.config) } } };
}

export function demoNodeRevisions(node: Node) {
  if (!histories.has(node.id)) {
    const entries: DemoRevision[] = [];
    if (node.current_revision > 0) entries.push(snapshot(node));
    if (node.current_revision > 1) entries.push({ revision: node.current_revision - 1, created_at: node.created_at, published_by: "adm_demo", restorable: false, config: { transport: "tcp" } });
    histories.set(node.id, entries);
  }
  return structuredClone(histories.get(node.id)!);
}

export function recordDemoNodeRevision(node: Node) {
  const entries = histories.get(node.id) ?? [];
  histories.set(node.id, [snapshot(node), ...entries.filter((entry) => entry.revision !== node.current_revision)]);
}

export function restoreDemoNodeRevision(node: Node, revision: number): Node | undefined {
  const entry = demoNodeRevisions(node).find((item) => item.revision === revision);
  if (!entry?.restorable || !entry.snapshot) return undefined;
  return { ...node, ...entry.snapshot.node, config: structuredClone(entry.snapshot.spec.settings), route_policy_id: null, admin_route_policy_id: null, member_route_policy_id: null, has_draft: true, restored_from_revision: revision, config_version: (node.config_version ?? 1) + 1, updated_at: new Date().toISOString() };
}
