import { useEffect, useRef, useState, type FormEvent } from "react";
import { Link, useLocation } from "wouter";
import { ArrowLeft, Save } from "lucide-react";
import { ApiError, api } from "../api";
import { Button, EmptyState, PageHeader, RequestFailure, StatusBadge, TableSkeleton, useToast } from "../components/ui";
import { VersionConflict } from "../components/VersionConflict";
import { creationIntent } from "../mutations";
import { ResourceFields } from "../components/ResourceFields";
import { useUnsavedChanges } from "../components/AdminNavigation";
import { safeReturnTo } from "../navigation";
import { requestErrorMessage, sameFormValues } from "../resourceModel";
import type { Resource, ResourceDefinition } from "../resourceDefinitions";
import { validateRouteRulesValue } from "../components/RouteRulesEditor";
import { validateOutboundSettingsValue } from "../components/OutboundConfigForm";

const sections: Record<string, { title: string; keys: string[] }[]> = {
  "/machines": [{ title: "基础信息", keys: ["name", "region", "host"] }, { title: "标签与备注", keys: ["labels", "notes"] }],
  "/users": [{ title: "角色与身份", keys: ["role", "uuid", "name", "email"] }, { title: "套餐与到期", keys: ["plan_id", "expires_mode", "expires_at"] }, { title: "门户账号与密码", keys: ["portal_login", "portal_password"] }, { title: "内部备注", keys: ["notes"] }],
  "/plans": [{ title: "基本信息与权限", keys: ["name", "access_group_id"] }, { title: "流量与限制", keys: ["traffic_gb", "reset_strategy", "speed_limit_mbps", "device_limit", "default_valid_days"] }, { title: "备注", keys: ["notes"] }],
  "/access-groups": [{ title: "基本信息", keys: ["name", "notes"] }, { title: "可访问节点", keys: ["node_ids"] }],
  "/route-policies": [{ title: "基础信息与范围", keys: ["name", "scope", "default_outbound_tag", "notes"] }, { title: "规则编排", keys: ["rules"] }],
  "/outbounds": [{ title: "名称与标记", keys: ["name", "tag"] }, { title: "协议与连接参数", keys: ["settings"] }],
};

export function ResourceEditorPage<T extends Resource>({ definition, id }: { definition: ResourceDefinition<T>; id?: string }) {
  const { title, endpoint, fields, createLabel, related } = definition;
  const latest = useRef(definition);
  latest.current = definition;
  const [, navigate] = useLocation();
  const base = endpoint === "/route-policies" ? "/routes" : endpoint;
  const [returnTo] = useState(() => safeReturnTo(base));
  const [record, setRecord] = useState<T | null>(null);
  const [values, setValues] = useState<Record<string, string>>({});
  const [initial, setInitial] = useState<Record<string, string>>({});
  const [loadState, setLoadState] = useState<"loading" | "ready" | "missing" | "error">(id ? "loading" : "ready");
  const [loadError, setLoadError] = useState("");
  const [requestError, setRequestError] = useState("");
  const [requestFailure, setRequestFailure] = useState<unknown>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [activeSection, setActiveSection] = useState(0);
  const initialized = useRef(false);
  const savingRef = useRef(false);
  const intent = useRef(creationIntent());
  const toast = useToast();
  const dirty = initialized.current && !sameFormValues(values, initial);
  const bypass = useUnsavedChanges(dirty, saving);

  useEffect(() => {
    if (!id) return;
    const controller = new AbortController();
    setLoadState("loading");
    api.get<T[]>(endpoint, controller.signal).then((records) => {
      if (controller.signal.aborted) return;
      if (!Array.isArray(records)) throw new Error("服务返回了无法识别的资源列表，请重试。");
      const target = records.find((item) => item.id === id);
      if (!target) { setLoadState("missing"); return; }
      const next = latest.current.toValues(target);
      setRecord(target); setValues(next); setInitial(next);
      initialized.current = true;
      setLoadState("ready");
    }).catch((reason) => {
      if (controller.signal.aborted) return;
      setLoadError(requestErrorMessage(reason, `${title}加载失败`));
      setLoadState("error");
    });
    return () => controller.abort();
  }, [attempt, endpoint, id, title]);

  useEffect(() => {
    if (id || initialized.current || related?.loading || related?.error) return;
    const next = { ...latest.current.defaults };
    initialized.current = true;
    setValues(next); setInitial(next);
  }, [endpoint, id, related?.error, related?.loading]);

  function focusError(next: Record<string, string>) {
    requestAnimationFrame(() => {
      const field = Array.from(document.querySelectorAll<HTMLElement>("[data-field-key]")).find((element) => element.dataset.fieldKey === Object.keys(next)[0]);
      field?.querySelector<HTMLElement>("input, select, textarea, button")?.focus();
    });
  }

  async function save(event: FormEvent) {
    event.preventDefault();
    if (savingRef.current || !initialized.current || related?.loading || related?.error) return;
    const next: Record<string, string> = {};
    fields.filter((field) => (!field.editOnly || record) && (!field.visible || field.visible(values))).forEach((field) => {
      if (field.required && !values[field.key]?.trim()) next[field.key] = `请填写${field.label}`;
      if (field.type === "email" && values[field.key]?.trim() && !/^\S+@\S+\.\S+$/.test(values[field.key].trim())) next[field.key] = "请填写有效的邮箱地址";
      if (field.type === "json" && values[field.key]) { try { JSON.parse(values[field.key]); } catch { next[field.key] = `${field.label}必须是有效 JSON`; } }
      if (field.type === "route-rules") { const error = validateRouteRulesValue(values[field.key] ?? "[]"); if (error) next[field.key] = error; }
      if (field.type === "outbound-config") { const error = validateOutboundSettingsValue(values.protocol ?? "", values.settings ?? "{}"); if (error) next[field.key] = error; }
    });
    setErrors(next); setRequestError(""); setRequestFailure(null);
    if (Object.keys(next).length) { focusError(next); return; }
    savingRef.current = true; setSaving(true);
    try {
      const payload = latest.current.transform(values, Boolean(id));
      if (id) {
        if (record?.config_version !== undefined) await api.patch(`${endpoint}/${id}`, payload, { configVersion: record.config_version });
        else await api.patch(`${endpoint}/${id}`, payload);
      } else await api.post(endpoint, payload, { idempotencyKey: intent.current(payload) });
      setInitial(values);
      toast(id ? `${title}已更新` : `${createLabel}已创建`);
      bypass(() => navigate(returnTo, { replace: true }));
    } catch (reason) {
      const fieldErrors = reason instanceof ApiError ? reason.fields : {};
      setErrors(fieldErrors); focusError(fieldErrors);
      setRequestError(requestErrorMessage(reason, "保存失败，请重试"));
      setRequestFailure(reason);
    } finally { savingRef.current = false; setSaving(false); }
  }

  const pageTitle = id ? `编辑${title}` : createLabel;
  const loading = loadState === "loading" || Boolean(related?.loading);
  const grouping = sections[endpoint] ?? [{ title: "配置", keys: fields.map((field) => field.key) }];
  const assigned = new Set(grouping.flatMap((section) => section.keys));
  const groups = [...grouping, ...(fields.some((field) => !assigned.has(field.key)) ? [{ title: "其他配置", keys: fields.filter((field) => !assigned.has(field.key)).map((field) => field.key) }] : [])];
  return <div className="page resource-page resource-editor-page">
    <div className="back-row"><Link to={returnTo}><ArrowLeft size={16} />返回{title}列表</Link></div>
    <PageHeader title={pageTitle} description={id ? "修改配置后保存，返回时保留原列表的筛选条件。" : `填写${title}配置，完成后保存。`} actions={record && <StatusBadge status={record.status} />} />
    {loading ? <TableSkeleton columns={2} rows={6} /> : loadState === "error" || related?.error ? <EmptyState title={`${title}加载失败`} description={loadError || related?.error || "关联数据加载失败"} action={<Button onClick={() => { setAttempt((value) => value + 1); related?.retry(); }}>重新加载</Button>} /> : loadState === "missing" ? <EmptyState title={`${title}不存在`} description="记录可能已被删除，请返回列表确认。" action={<Link className="button button--secondary" to={returnTo}>返回列表</Link>} /> : <div className="editor-layout">
      <nav className="editor-section-nav" aria-label={`${title}编辑章节`}>{groups.map((section, index) => <button key={section.title} type="button" aria-current={activeSection === index ? "location" : undefined} onClick={() => { setActiveSection(index); document.getElementById(`resource-section-${index}`)?.scrollIntoView({ behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "instant" : "smooth", block: "start" }); }}>{section.title}</button>)}</nav>
      <form className={`editor-form ${["/route-policies", "/outbounds"].includes(endpoint) ? "" : "editor-form--standard"}`} onSubmit={(event) => void save(event)} noValidate aria-label={pageTitle} aria-busy={saving}>
        {requestError && <RequestFailure message={requestError} error={requestFailure} />}
        {requestFailure instanceof ApiError && ["CONFIG_VERSION_CONFLICT", "EDIT_VERSION_CONFLICT"].includes(requestFailure.code) && <VersionConflict version={record?.config_version} onReload={() => { setRequestError(""); setRequestFailure(null); setErrors({}); setAttempt((value) => value + 1); }} />}
        <fieldset disabled={saving} className="editor-fields">
        {groups.map((section, index) => <section className="form-section" id={`resource-section-${index}`} key={section.title}><div className="form-section__heading"><h2>{section.title}</h2></div><ResourceFields fields={fields.filter((field) => section.keys.includes(field.key))} record={record} values={values} errors={errors} onChange={(key, value) => { setValues((current) => ({ ...current, [key]: value })); setErrors((current) => ({ ...current, [key]: "" })); }} />{section.keys.some((key) => ["plan_id", "access_group_id", "node_ids"].includes(key)) && definition.editorContext?.(values, record)}</section>)}
        </fieldset>
        <footer className="editor-actions"><span className="editor-status" role="status">{saving ? "正在保存…" : dirty ? "有未保存修改" : id ? "已加载当前配置" : "填写后保存"}</span><div><Button type="button" variant="ghost" disabled={saving} onClick={() => navigate(returnTo, { replace: true })}>取消</Button><Button type="submit" variant="primary" loading={saving}><Save size={16} />{id ? "保存修改" : "创建"}</Button></div></footer>
      </form>
    </div>}
  </div>;
}
