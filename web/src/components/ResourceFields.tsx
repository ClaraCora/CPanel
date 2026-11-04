import type { FieldSpec, Resource } from "../resourceDefinitions";
import { Field } from "./ui";
import { NodeMultiSelect } from "./NodeMultiSelect";
import { RouteRulesEditor } from "./RouteRulesEditor";
import { OutboundConfigForm, parseOutboundSettings } from "./OutboundConfigForm";

export function ResourceFields({ fields, record, values, errors, onChange }: {
  fields: FieldSpec[];
  record: Resource | null;
  values: Record<string, string>;
  errors: Record<string, string>;
  onChange: (key: string, value: string) => void;
}) {
  return <div className="form-grid">{fields.filter((field) => (!field.editOnly || record) && (!field.visible || field.visible(values))).map((field) => {
    const group = ["node-picker", "route-rules", "outbound-config"].includes(field.type || "");
    const full = group || ["textarea", "json"].includes(field.type || "");
    const disabled = field.disabled?.(record) ?? false;
    return <div key={field.key} data-field-key={field.key} className={full ? "field--full" : undefined}>
      <Field label={field.label} required={field.required} helper={field.helper} error={errors[field.key]} group={group}>
        {field.type === "textarea" || field.type === "json" ?
          <textarea disabled={disabled} className={field.type === "json" ? "code-editor" : undefined} rows={field.type === "json" ? 10 : 4} value={values[field.key] ?? ""} placeholder={field.placeholder} onChange={(event) => onChange(field.key, event.target.value)} /> :
          field.type === "select" ?
            <select disabled={Boolean((record && field.lockedValue && values[field.key] === field.lockedValue) || disabled)} value={values[field.key] ?? ""} onChange={(event) => onChange(field.key, event.target.value)}>{values[field.key] && !field.options?.some((option) => option.value === values[field.key]) && <option value={values[field.key]}>未找到关联资源（{values[field.key]}）</option>}{field.options?.filter((option) => (!option.editOnly || record) && (!option.exclusive || values[field.key] === option.value)).map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}</select> :
            field.type === "node-picker" ?
              <NodeMultiSelect value={values[field.key] ?? ""} options={field.options ?? []} onChange={(value) => onChange(field.key, value)} /> :
              field.type === "route-rules" ?
                <RouteRulesEditor value={values[field.key] ?? "[]"} outbounds={field.routeOutbounds ?? []} onChange={(value) => onChange(field.key, value)} /> :
                field.type === "outbound-config" ?
                  <OutboundConfigForm protocol={values.protocol ?? "socks"} settings={parseOutboundSettings(values.settings ?? "{}")} proxyTag={values.proxy_tag ?? ""} outbounds={field.routeOutbounds ?? []} currentID={record?.id} onProtocolChange={(protocol, settings) => { onChange("protocol", protocol); onChange("settings", JSON.stringify(settings)); }} onSettingsChange={(settings) => onChange("settings", JSON.stringify(settings))} onProxyTagChange={(value) => onChange("proxy_tag", value)} /> :
                  <input disabled={disabled} type={field.type ?? "text"} autoComplete={field.type === "password" ? "new-password" : undefined} value={values[field.key] ?? ""} placeholder={field.placeholder} onChange={(event) => onChange(field.key, event.target.value)} />}
      </Field>
    </div>;
  })}</div>;
}
