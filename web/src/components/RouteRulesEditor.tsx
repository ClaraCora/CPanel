import { ArrowDown, ArrowUp, Plus, Trash2, X } from "lucide-react";
import type { Outbound, RoutePolicyRule } from "../types";

type MatchKey = keyof RoutePolicyRule["match"];

const matchOptions: { value: MatchKey; label: string; placeholder: string; multiline?: boolean }[] = [
  { value: "domains", label: "精确域名", placeholder: "api.example.com, cdn.example.com" },
  { value: "domain_suffixes", label: "域名后缀", placeholder: "example.com, example.net" },
  { value: "domain_regexes", label: "域名正则", placeholder: "每行一条，可直接粘贴 regexp: 开头的 Xray 规则", multiline: true },
  { value: "geo_ips", label: "GeoIP 分类", placeholder: "google, cn" },
  { value: "ip_cidrs", label: "目标 IP/CIDR", placeholder: "10.0.0.0/8, 2001:db8::/32" },
  { value: "ports", label: "目标端口", placeholder: "443, 8000-9000" },
  { value: "networks", label: "网络协议", placeholder: "" },
  { value: "source_cidrs", label: "来源 IP/CIDR", placeholder: "192.168.1.0/24" },
  { value: "source_ports", label: "来源端口", placeholder: "1024-65535" },
];

const geoIPPresetOptions = ["google", "cn", "private"];

const emptyRule = (): RoutePolicyRule => ({ name: "", match: { domain_suffixes: [] }, action: { type: "direct" } });

export function RouteRulesEditor({ value, outbounds, onChange }: { value: string; outbounds: Outbound[]; onChange: (value: string) => void }) {
  const rules = parseRouteRules(value);
  const availableOutbounds = outbounds.filter((item) => item.status === "active" && !["direct", "block"].includes(item.protocol) && item.kernel_support.includes("xray"));

  function commit(next: RoutePolicyRule[]) {
    onChange(JSON.stringify(next));
  }

  function updateRule(index: number, updater: (rule: RoutePolicyRule) => RoutePolicyRule) {
    commit(rules.map((rule, position) => position === index ? updater(rule) : rule));
  }

  function move(index: number, direction: -1 | 1) {
    const target = index + direction;
    if (target < 0 || target >= rules.length) return;
    const next = [...rules];
    [next[index], next[target]] = [next[target], next[index]];
    commit(next);
  }

  return <div className="route-builder">
    <header className="route-builder__header">
      <div><strong>匹配规则</strong><span>{rules.length} 条 · 按顺序执行</span></div>
      <button type="button" className="button button--secondary" onClick={() => commit([...rules, emptyRule()])}><Plus size={15} />添加规则</button>
    </header>
    {rules.length === 0 ? <div className="route-builder__empty"><span>暂无匹配规则</span><button type="button" onClick={() => commit([emptyRule()])}>添加第一条规则</button></div> : <div className="route-builder__rules">
      {rules.map((rule, index) => <article className="route-rule" key={index}>
        <header className="route-rule__header">
          <span className="route-rule__index">{index + 1}</span>
          <input aria-label={`第 ${index + 1} 条规则名称`} value={rule.name ?? ""} placeholder="规则名称，例如：广告域名阻断" onChange={(event) => updateRule(index, (current) => ({ ...current, name: event.target.value }))} />
          <label className="route-rule__toggle"><input type="checkbox" checked={!rule.disabled} onChange={(event) => updateRule(index, (current) => ({ ...current, disabled: !event.target.checked }))} /><span>{rule.disabled ? "停用" : "启用"}</span></label>
          <div className="route-rule__tools">
            <button type="button" className="icon-button" disabled={index === 0} aria-label="上移规则" title="上移" onClick={() => move(index, -1)}><ArrowUp size={15} /></button>
            <button type="button" className="icon-button" disabled={index === rules.length - 1} aria-label="下移规则" title="下移" onClick={() => move(index, 1)}><ArrowDown size={15} /></button>
            <button type="button" className="icon-button icon-button--danger" aria-label="删除规则" title="删除规则" onClick={() => commit(rules.filter((_, position) => position !== index))}><Trash2 size={15} /></button>
          </div>
        </header>
        <div className="route-rule__body">
          <section className="route-rule__matches">
            <h4>匹配条件</h4>
            {matchOptions.filter((option) => option.value in rule.match).map((option) => <div className={`route-match-row ${option.multiline ? "route-match-row--multiline" : ""}`} key={option.value}>
              <span>{option.label}</span>
              {option.value === "networks" ? <div className="route-network-options">
                {["tcp", "udp"].map((network) => <label key={network}><input type="checkbox" checked={(rule.match.networks ?? []).includes(network)} onChange={() => updateRule(index, (current) => ({ ...current, match: { ...current.match, networks: toggleValue(current.match.networks ?? [], network) } }))} /><span>{network.toUpperCase()}</span></label>)}
              </div> : option.value === "geo_ips" ? <div className="route-geoip-field">
                <input spellCheck={false} value={(rule.match.geo_ips ?? []).join(", ")} placeholder={option.placeholder} onChange={(event) => updateRule(index, (current) => ({ ...current, match: { ...current.match, geo_ips: splitValues(event.target.value) } }))} />
                <div className="route-geoip-presets" role="group" aria-label="常用 GeoIP 分类">{geoIPPresetOptions.map((category) => {
                  const selected = (rule.match.geo_ips ?? []).map(normalizeGeoIPCategory).includes(category);
                  return <button type="button" aria-pressed={selected} className={selected ? "active" : ""} title={`${selected ? "移除" : "添加"} ${category}`} onClick={() => updateRule(index, (current) => ({ ...current, match: { ...current.match, geo_ips: toggleGeoIPCategory(current.match.geo_ips ?? [], category) } }))} key={category}>{category}</button>;
                })}</div>
              </div> : option.multiline ? <textarea rows={6} spellCheck={false} value={(rule.match[option.value] ?? []).join("\n")} placeholder={option.placeholder} onChange={(event) => updateRule(index, (current) => ({ ...current, match: { ...current.match, [option.value]: splitRegexValues(event.target.value) } }))} /> : <input spellCheck={false} value={(rule.match[option.value] ?? []).join(", ")} placeholder={option.placeholder} onChange={(event) => updateRule(index, (current) => ({ ...current, match: { ...current.match, [option.value]: splitValues(event.target.value) } }))} />}
              <button type="button" className="icon-button" aria-label={`移除${option.label}`} title="移除条件" onClick={() => updateRule(index, (current) => {
                const match = { ...current.match };
                delete match[option.value];
                return { ...current, match };
              })}><X size={15} /></button>
            </div>)}
            {matchOptions.some((option) => !(option.value in rule.match)) && <label className="route-add-match"><Plus size={14} /><select aria-label="添加匹配条件" value="" onChange={(event) => {
              const key = event.target.value as MatchKey;
              if (key) updateRule(index, (current) => ({ ...current, match: { ...current.match, [key]: [] } }));
            }}><option value="">添加匹配条件</option>{matchOptions.filter((option) => !(option.value in rule.match)).map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select></label>}
          </section>
          <section className="route-rule__action">
            <h4>执行动作</h4>
            <div className="route-action-options" role="group" aria-label="执行动作">
              {([{"value":"direct","label":"直连"},{"value":"block","label":"阻断"},{"value":"route","label":"指定出站"}] as const).map((action) => <button type="button" className={rule.action.type === action.value ? "active" : ""} aria-pressed={rule.action.type === action.value} disabled={action.value === "route" && availableOutbounds.length === 0} title={action.value === "route" && availableOutbounds.length === 0 ? "请先添加可用于 Xray 的出站" : undefined} onClick={() => updateRule(index, (current) => ({ ...current, action: { type: action.value, ...(action.value === "route" && availableOutbounds[0] ? { target: availableOutbounds[0].tag } : {}) } }))} key={action.value}>{action.label}</button>)}
            </div>
            {rule.action.type === "route" && <label className="route-outbound-select"><span>出站目标</span><select value={rule.action.target ?? ""} onChange={(event) => updateRule(index, (current) => ({ ...current, action: { type: "route", target: event.target.value } }))}><option value="">请选择出站</option>{availableOutbounds.map((item) => <option value={item.tag} key={item.id}>{item.name} · {item.tag}</option>)}</select></label>}
          </section>
        </div>
      </article>)}
    </div>}
  </div>;
}

export function validateRouteRulesValue(value: string): string {
  const rules = parseRouteRules(value);
  for (let index = 0; index < rules.length; index++) {
    const rule = rules[index];
    if (rule.disabled) continue;
    if (!Object.values(rule.match).some((values) => values && values.length > 0)) return `第 ${index + 1} 条规则至少需要一个匹配条件`;
    for (const value of rule.match.geo_ips ?? []) {
      const category = normalizeGeoIPCategory(value);
      if (!/^[a-z0-9][a-z0-9._-]*$/.test(category)) return `第 ${index + 1} 条规则包含无效 GeoIP 分类：${value}`;
    }
    if (rule.action.type === "route" && !rule.action.target) return `第 ${index + 1} 条规则请选择出站目标`;
  }
  return "";
}

function parseRouteRules(value: string): RoutePolicyRule[] {
  try {
    const parsed = JSON.parse(value || "[]");
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function splitValues(value: string): string[] {
  return value.split(/[,，\n]/).map((item) => item.trim()).filter(Boolean);
}

function splitRegexValues(value: string): string[] {
  return value.split(/\r?\n/).map((item) => item.trim().replace(/^regexp:\s*/i, "").replace(/\\\\/g, "\\")).filter(Boolean);
}

function toggleValue(values: string[], value: string): string[] {
  return values.includes(value) ? values.filter((item) => item !== value) : [...values, value];
}

function normalizeGeoIPCategory(value: string): string {
  return value.trim().toLowerCase().replace(/^geoip:\s*/i, "");
}

function toggleGeoIPCategory(values: string[], category: string): string[] {
  const normalized = values.map(normalizeGeoIPCategory).filter(Boolean);
  return normalized.includes(category) ? normalized.filter((value) => value !== category) : [...normalized, category];
}
