import { useMemo, useState } from "react";
import { Copy, Eye, Filter, LoaderCircle, MapPin, RefreshCw, Search } from "lucide-react";
import { api } from "../api";
import { Button, Drawer, EmptyState, PageHeader, StatusBadge, TableSkeleton, formatPreciseDate, useToast } from "../components/ui";
import { useResource } from "../hooks";
import type { UserAccessIPAddress, UserAccessIPLocation, UserAccessIPLocationResult, UserAccessIPAccount } from "../types";

const roleLabels = { admin: "管理员", user: "用户", friend: "朋友" } as const;

type LocationRequestState = {
  loading: boolean;
  error: string;
  location?: UserAccessIPLocation;
};

export function UserAccessIPsPage() {
  const { data, loading, error, reload } = useResource<UserAccessIPAccount[]>("/user-access-ips", [], 30_000);
  const [query, setQuery] = useState("");
  const [role, setRole] = useState("all");
  const [selected, setSelected] = useState<UserAccessIPAccount | null>(null);
  const [locationRequests, setLocationRequests] = useState<Record<string, LocationRequestState>>({});
  const toast = useToast();
  const filtered = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    return data.filter((item) => {
      if (role !== "all" && item.role !== role) return false;
      if (!keyword) return true;
      const values = [item.user_name, item.user_id, ...item.addresses.flatMap((address) => {
        const location = locationRequests[address.ip_address]?.location ?? address.location;
        return [address.ip_address, address.last_node_name, address.last_node_id ?? "", location?.country ?? "", location?.province ?? "", location?.city ?? "", location?.isp ?? ""];
      })];
      return values.some((value) => value.toLowerCase().includes(keyword));
    });
  }, [data, locationRequests, query, role]);

  async function lookupLocation(ipAddress: string, refresh = false) {
    if (locationRequests[ipAddress]?.loading) return;
    setLocationRequests((current) => ({
      ...current,
      [ipAddress]: { ...current[ipAddress], loading: true, error: "" },
    }));
    try {
      const result = await api.post<UserAccessIPLocationResult>("/user-access-ips/location", { ip_address: ipAddress, refresh });
      setLocationRequests((current) => ({
        ...current,
        [ipAddress]: { loading: false, error: "", location: result.location },
      }));
      toast(result.cached ? "已显示缓存的 IP 归属地" : refresh ? "IP 归属地已更新" : "IP 归属地已获取");
    } catch (reason) {
      const message = reason instanceof Error ? reason.message : "归属地查询失败，请稍后重试";
      setLocationRequests((current) => ({
        ...current,
        [ipAddress]: { ...current[ipAddress], loading: false, error: message },
      }));
      toast(message, "error");
    }
  }

  async function copyIP(value: string) {
    try {
      await writeClipboard(value);
      toast("IP 地址已复制");
    } catch {
      toast("复制失败，请手动选择 IP 地址", "error");
    }
  }

  return <div className="page">
    <PageHeader title="用户访问 IP" description="节点上报的账号源 IP，每个账号保留最近使用的 10 个不同地址" actions={<Button type="button" disabled={loading} onClick={() => void reload()}><RefreshCw size={16} />刷新</Button>} />
    <div className="toolbar">
      <label className="search-box"><Search size={16} /><span className="sr-only">搜索用户访问 IP</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索账号、用户 ID、IP、地区或节点" /></label>
      <div className="toolbar__filters"><Filter size={15} /><label className="sr-only" htmlFor="access-ip-role">筛选账号分类</label><select id="access-ip-role" value={role} onChange={(event) => setRole(event.target.value)}><option value="all">全部分类</option><option value="admin">管理员</option><option value="user">用户</option><option value="friend">朋友</option></select></div>
      <span className="toolbar__count">{filtered.length} 个账号</span>
    </div>
    <div className="table-surface access-ip-table">{loading ? <TableSkeleton columns={8} rows={7} /> : error ? <EmptyState title="访问 IP 加载失败" description={error} action={<Button onClick={() => void reload()}>重新加载</Button>} /> : filtered.length === 0 ? <EmptyState title={data.length === 0 ? "暂无用户访问 IP" : "没有匹配的访问 IP"} description={data.length === 0 ? "尚未收到节点上报的账号源 IP。" : "请调整搜索内容或账号分类。"} /> : <div className="table-scroll"><table><thead><tr><th>账号</th><th>分类</th><th>状态</th><th>最近 IP</th><th>记录数</th><th>最后节点</th><th>最后访问</th><th className="col-access-ip-action"><span className="sr-only">操作</span></th></tr></thead><tbody>{filtered.map((item) => {
      const latest = item.addresses[0];
      return <tr key={item.user_id}><td><strong className="cell-primary">{item.user_name}</strong><span className="resource-id">{item.user_id}</span></td><td>{roleLabels[item.role]}</td><td><StatusBadge status={item.status} /></td><td><IPAddressLocation address={latest} request={locationRequests[latest.ip_address]} onLookup={lookupLocation} /></td><td><span className="access-ip-count"><strong>{item.addresses.length}</strong><small>/ 10</small></span></td><td><strong className="cell-primary">{latest.last_node_name || "节点已删除"}</strong>{latest.last_node_id && <span className="resource-id">{latest.last_node_id}</span>}</td><td className="mono">{formatPreciseDate(item.last_seen_at)}</td><td className="col-access-ip-action"><button type="button" className="icon-button" aria-label={`查看 ${item.user_name} 的访问 IP`} title="查看最近 IP" onClick={() => setSelected(item)}><Eye size={16} /></button></td></tr>;
    })}</tbody></table></div>}</div>
    <Drawer wide open={Boolean(selected)} title={selected ? `${selected.user_name} 的访问 IP` : "访问 IP"} description={selected ? `${roleLabels[selected.role]} · ${selected.user_id}` : undefined} onClose={() => setSelected(null)}>{selected && <div className="access-ip-drawer"><div className="access-ip-drawer__summary"><span>最近访问</span><strong className="mono">{formatPreciseDate(selected.last_seen_at)}</strong><span>已记录</span><strong className="mono">{selected.addresses.length} / 10</strong></div><div className="table-scroll"><table><thead><tr><th>IP 地址</th><th>首次出现</th><th>最后出现</th><th>最后节点</th><th><span className="sr-only">复制</span></th></tr></thead><tbody>{selected.addresses.map((address) => <tr key={address.ip_address}><td><IPAddressLocation address={address} request={locationRequests[address.ip_address]} onLookup={lookupLocation} /></td><td className="mono">{formatPreciseDate(address.first_seen_at)}</td><td className="mono">{formatPreciseDate(address.last_seen_at)}</td><td><strong className="cell-primary">{address.last_node_name || "节点已删除"}</strong>{address.last_node_id && <span className="resource-id">{address.last_node_id}</span>}</td><td><button type="button" className="icon-button" aria-label={`复制 IP ${address.ip_address}`} title="复制 IP" onClick={() => void copyIP(address.ip_address)}><Copy size={16} /></button></td></tr>)}</tbody></table></div></div>}</Drawer>
  </div>;
}

function IPAddressLocation({ address, request, onLookup }: { address: UserAccessIPAddress; request?: LocationRequestState; onLookup: (ipAddress: string, refresh?: boolean) => Promise<void> }) {
  const location = request?.location ?? address.location ?? undefined;
  const locationLabel = location ? formatLocation(location) : "";
  return <div className="access-ip-location">
    <code className="access-ip-address">{address.ip_address}</code>
    <div className="access-ip-location__status" aria-live="polite">
      {location ? <div className="access-ip-location__result">
        <span className="access-ip-location__text" title={`${locationLabel} · 查询于 ${formatPreciseDate(location.resolved_at)}`}><MapPin size={13} aria-hidden="true" /><span>{locationLabel}</span></span>
        <button type="button" className="access-ip-location__refresh" disabled={request?.loading} aria-label={`重新查询 ${address.ip_address} 的归属地`} title="重新查询归属地" onClick={() => void onLookup(address.ip_address, true)}>{request?.loading ? <LoaderCircle className="spin" size={13} aria-hidden="true" /> : <RefreshCw size={13} aria-hidden="true" />}</button>
      </div> : <button type="button" className="access-ip-location__lookup" disabled={request?.loading} aria-label={`${request?.error ? "重试查询" : "获取"} ${address.ip_address} 的归属地`} title="获取 IP 归属地" onClick={() => void onLookup(address.ip_address)}>{request?.loading ? <LoaderCircle className="spin" size={13} aria-hidden="true" /> : <MapPin size={13} aria-hidden="true" />}{request?.loading ? "查询中" : request?.error ? "重试获取地区" : "获取地区"}</button>}
      {request?.error && <span className="access-ip-location__error" role="alert">{request.error}</span>}
    </div>
  </div>;
}

function formatLocation(location: UserAccessIPLocation) {
  const values = location.scope === "private" ? [location.country] : [location.country, location.province, location.city, location.isp];
  return values.reduce<string[]>((result, value) => {
    const normalized = value.trim();
    if (normalized && !result.includes(normalized)) result.push(normalized);
    return result;
  }, []).join(" · ") || "地区未知";
}

async function writeClipboard(value: string) {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(value);
      return;
    } catch {
      // Non-HTTPS deployments use the selection fallback below.
    }
  }
  const input = document.createElement("textarea");
  input.value = value;
  input.style.position = "fixed";
  input.style.opacity = "0";
  document.body.append(input);
  input.select();
  const copied = document.execCommand("copy");
  input.remove();
  if (!copied) throw new Error("clipboard unavailable");
}
