import { Fragment, useId } from "react";
import type { FieldSpec } from "../resourceDefinitions";
import { outboundProtocolOptions, outboundTarget, parseOutboundSettings } from "./OutboundConfigForm";

export type SummaryGroup = { title: string; rows: [string, string][] };

export function ConfigSummary({ groups }: { groups: SummaryGroup[] }) {
  const heading = useId();
  return <aside className="editor-summary" aria-labelledby={heading}>
    <h2 id={heading}>配置摘要</h2>
    {groups.filter((group) => group.rows.length > 0).map((group) => <section key={group.title}>
      <h3>{group.title}</h3>
      <dl>{group.rows.map(([label, value]) => <Fragment key={label}><dt>{label}</dt><dd>{value || "未填写"}</dd></Fragment>)}</dl>
    </section>)}
  </aside>;
}

export function resourceSummary(endpoint: string, values: Record<string, string>, fields: FieldSpec[]): SummaryGroup[] {
  function rows(keys: string[]): [string, string][] {
    return keys.flatMap((key): [string, string][] => {
      const field = fields.find((candidate) => candidate.key === key);
      if (!field || (field.visible && !field.visible(values))) return [];
      const value = values[key] ?? "";
      const display = field.options?.find((option) => option.value === value)?.label ?? value;
      return [[field.label, display]];
    });
  }
  // Project only the fields used for review. Credentials, notes and raw outbound settings stay in the editor.
  if (endpoint === "/users") return [
    { title: "账户资料", rows: rows(["name", "role", "email", "portal_login"]) },
    { title: "订阅期限", rows: rows(["plan_id", "expires_mode", "expires_at"]) },
  ];
  if (endpoint === "/plans") return [
    { title: "套餐与授权", rows: rows(["name", "access_group_id"]) },
    { title: "限额与周期", rows: rows(["traffic_gb", "reset_strategy", "speed_limit_mbps", "device_limit", "default_valid_days"]) },
  ];
  if (endpoint === "/outbounds") return [
    { title: "出站标识", rows: rows(["name", "tag"]) },
    { title: "连接方式", rows: [
      ["协议", outboundProtocolOptions.find((option) => option.id === values.protocol)?.label ?? values.protocol ?? ""],
      ["连接目标", outboundTarget(values.protocol, parseOutboundSettings(values.settings || "{}"))],
      ["上游出站", values.proxy_tag || "直接连接目标"],
    ] },
  ];
  return [];
}
