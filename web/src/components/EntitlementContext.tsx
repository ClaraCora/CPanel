import { Link } from "wouter";
import { Button, formatBytes } from "./ui";
import { useResource } from "../hooks";
import { editorHref } from "../navigation";
import type { AccessGroup, Node, Plan, Setting, User } from "../types";

function NodeScope({ group, nodes }: { group: AccessGroup; nodes: Node[] }) {
  const bound = (group.node_ids ?? []).map((id) => ({ id, node: nodes.find((item) => item.id === id) }));
  const published = bound.filter(({ node }) => node?.status === "published").length;
  return <><p>绑定 {bound.length} 个节点 · 已发布 {published} 个{group.status !== "active" && " · 权限组已停用，当前不授予访问"}</p><ul className="entitlement-nodes">{bound.map(({ id, node }) => <li key={id}><Link to={editorHref("/nodes", id)}>{node?.name || id}</Link><span>{!node ? "节点不存在" : node.status === "published" ? "已发布" : node.status === "draft" ? "草稿，尚未提供订阅" : node.status === "disabled" ? "已停用" : "状态异常"}</span></li>)}</ul></>;
}

export function GroupContext({ group }: { group?: AccessGroup }) {
  const { data: nodes, loading, error, reload } = useResource<Node[]>("/nodes", []);
  if (!group) return null;
  return <aside className="entitlement-context" aria-label="权限组生效范围"><h3>节点访问范围</h3><Link to={editorHref("/access-groups", group.id)}>{group.name}</Link>{loading ? <p>正在读取节点状态…</p> : error ? <p>节点状态加载失败，不能计算已发布数量。<Button type="button" onClick={() => void reload()}>重试</Button></p> : <NodeScope group={group} nodes={nodes} />}</aside>;
}

function defaultExpiryDate(days: number, timeZone: string) {
  const parts = new Intl.DateTimeFormat("en-US", { timeZone, year: "numeric", month: "numeric", day: "numeric" }).formatToParts(new Date());
  const part = (type: Intl.DateTimeFormatPartTypes) => Number(parts.find((item) => item.type === type)?.value);
  const date = new Date(Date.UTC(part("year"), part("month") - 1, part("day") + days));
  return new Intl.DateTimeFormat("zh-CN", { timeZone: "UTC", year: "numeric", month: "2-digit", day: "2-digit" }).format(date);
}

export function UserEntitlementContext({ values, record, plans }: { values: Record<string, string>; record: User | null; plans: Plan[] }) {
  const groups = useResource<AccessGroup[]>("/access-groups", []);
  const nodes = useResource<Node[]>("/nodes", []);
  const site = useResource<Setting[]>("/settings/site", []);
  const plan = plans.find((item) => item.id === values.plan_id);
  const groupID = record?.access_group_override_id || plan?.access_group_id;
  const group = (Array.isArray(groups.data) ? groups.data : []).find((item) => item.id === groupID);
  const timeZone = String((Array.isArray(site.data) ? site.data : []).find((item) => item.key === "timezone")?.value || "Asia/Shanghai");
  let expiry = "";
  if (values.expires_mode === "default" || (values.expires_mode === "keep" && !record?.plan_id && !record?.expires_at && values.plan_id)) {
    if (!plan) expiry = "未分配套餐，默认期限为长期有效。";
    else if (!plan.default_valid_days) expiry = "套餐默认长期有效。";
    else if (site.loading) expiry = "正在读取站点时区…";
    else if (site.error) expiry = "站点时区读取失败，保存时由服务器计算套餐默认期限。";
    else {
      try { expiry = `套餐默认 ${plan.default_valid_days} 天，预计到期日期 ${defaultExpiryDate(plan.default_valid_days, timeZone)}（${timeZone}）。准确到期时间在保存时由服务器计算。`; }
      catch { expiry = "站点时区无效，请先在系统设置中修正。"; }
    }
  } else if (values.expires_mode === "permanent") expiry = "保存后长期有效。";
  else if (values.expires_mode === "keep") expiry = record?.expires_at ? `保持当前到期时间：${new Date(record.expires_at).toLocaleString("zh-CN")}。更换套餐不会自动延长。` : "保持当前长期有效期限。更换套餐不会自动重新计算。";
  else expiry = values.expires_at ? `自定义到期：${new Date(values.expires_at).toLocaleString("zh-CN")}（浏览器本地时间）。` : "请选择自定义到期时间。";
  const blocked = record && (record.status !== "active" ? "账号当前未启用" : record.expires_at && new Date(record.expires_at).getTime() <= Date.now() ? "账号当前已到期" : record.traffic_limit_bytes > 0 && record.traffic_used_bytes >= record.traffic_limit_bytes ? `流量已用尽（${formatBytes(record.traffic_used_bytes)} / ${formatBytes(record.traffic_limit_bytes)}）` : "");
  return <aside className="entitlement-context" aria-label="账号授权与到期预览"><h3>到期预览</h3><p role="status">{expiry}</p><h3>套餐与节点范围</h3>{plan ? <p><Link to={editorHref("/plans", plan.id)}>{plan.name}</Link>{plan.status !== "active" && " · 已停用，仅保留原关联"}</p> : <p>尚未分配套餐。</p>}{blocked && <p className="text-warning">当前授权受限：{blocked}。修改后以保存结果为准。</p>}{groups.loading || nodes.loading ? <p>正在读取权限组及节点…</p> : groups.error || nodes.error || !Array.isArray(groups.data) || !Array.isArray(nodes.data) ? <p>关联状态加载失败，未计算节点范围。<Button type="button" onClick={() => { void groups.reload(); void nodes.reload(); }}>重试</Button></p> : group ? <><p>生效权限组：<Link to={editorHref("/access-groups", group.id)}>{group.name}</Link>{record?.access_group_override_id && "（账号覆盖）"}</p><NodeScope group={group} nodes={nodes.data} /></> : <p>没有可用的权限组关联，暂不提供节点授权。</p>}</aside>;
}
