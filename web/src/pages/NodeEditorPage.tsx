import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import {
  ArrowLeft,
  CheckCircle2,
  Info,
  Plus,
  Rocket,
  Save,
  SlidersHorizontal,
  Trash2,
} from "lucide-react";
import { Link, useLocation, useParams, useSearchParams } from "wouter";
import { ApiError, api } from "../api";
import {
  Button,
  EmptyState,
  Field,
  PageHeader,
  ErrorDiagnostic,
  TableSkeleton,
  useToast,
} from "../components/ui";
import {
  defaultNodeSettings,
  NodeConfigForm,
  protocolSupported,
  validateNodeSettings,
  type NodeSettings,
} from "../components/NodeConfigForm";
import { useResource } from "../hooks";
import type { Machine, Node, NodeEndpoint, RoutePolicy, Setting } from "../types";
import { useUnsavedChanges } from "../components/AdminNavigation";
import { safeReturnTo } from "../navigation";
import { requestErrorMessage } from "../resourceModel";
import { prepareHysteriaTLS } from "../nodeTLS";
import { creationIntent } from "../mutations";
import { VersionConflict } from "../components/VersionConflict";

type NodeForm = {
  name: string;
  machine_id: string;
  route_policy_id: string;
  admin_route_policy_id: string;
  member_route_policy_id: string;
  protocol: string;
  listen_ip: string;
  server_port: string;
  kernel_type: string;
  config: NodeSettings;
  endpoints: NodeEndpoint[];
};
const emptyForm: NodeForm = {
  name: "",
  machine_id: "",
  route_policy_id: "",
  admin_route_policy_id: "",
  member_route_policy_id: "",
  protocol: "vless",
  listen_ip: "0.0.0.0",
  server_port: "443",
  kernel_type: "xray",
  config: defaultNodeSettings,
  endpoints: [],
};

export function NodeEditorPage() {
  const { id } = useParams();
  const [params] = useSearchParams();
  return <NodeEditor key={`${id || "new"}:${params.get("copy") || ""}`} />;
}

function NodeEditor() {
  const { id } = useParams();
  const [searchParams] = useSearchParams();
  const copyID = searchParams.get("copy");
  const editing = Boolean(id);
  const copying = !editing && Boolean(copyID);
  const [, navigate] = useLocation();
  const toast = useToast();
  const { data: machines, loading: machinesLoading, error: machinesError, reload: reloadMachines } = useResource<Machine[]>(
    "/machines",
    [],
  );
  const { data: routes, loading: routesLoading, error: routesError, reload: reloadRoutes } = useResource<RoutePolicy[]>("/route-policies", []);
  const { data: nodeDefaults, loading: defaultsLoading, error: defaultsError, reload: reloadDefaults } = useResource<Setting[]>("/settings/node_defaults", []);
  const initialized = useRef(false);
  const savingRef = useRef(false);
  const savedID = useRef(id || "");
  const configVersion = useRef<number | undefined>(undefined);
  const intent = useRef(creationIntent());
  const publishIntent = useRef(creationIntent());
  const pendingPublishIntent = useRef(typeof history.state?.nodePublishIntent === "string" ? history.state.nodePublishIntent as string : "");
  const [returnTo] = useState(() => safeReturnTo("/nodes"));
  const [initial, setInitial] = useState("");
  const [loadError, setLoadError] = useState("");
  const [missing, setMissing] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [pendingPublish, setPendingPublish] = useState(Boolean(history.state?.nodePublishFailed));
  const [form, setForm] = useState<NodeForm>(emptyForm);
  const [activeSection, setActiveSection] = useState("node-endpoints");
  const [nodeLoading, setNodeLoading] = useState(editing || copying);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [requestError, setRequestError] = useState(history.state?.nodePublishFailed ? "配置已保存，发布失败。请重试发布；无需重新创建节点。" : "");
  const [saving, setSaving] = useState<"save" | "publish" | "">("");
  const [requestFailure, setRequestFailure] = useState<unknown>(null);

  const dirty = initialized.current && JSON.stringify(form) !== initial;
  const bypass = useUnsavedChanges(dirty, Boolean(saving));
  const relatedError = machinesError || routesError || defaultsError;
  const relatedLoading = machinesLoading || routesLoading || defaultsLoading;

  useEffect(() => {
    const sourceID = id || copyID;
    if (!sourceID) return;
    const controller = new AbortController();
    setNodeLoading(true); setLoadError(""); setMissing(false);
    api.get<Node>(`/nodes/${sourceID}`, controller.signal).then((node) => {
      if (controller.signal.aborted) return;
      configVersion.current = copying ? undefined : node.config_version;
      const next: NodeForm = {
        name: copying ? `${node.name} 副本` : node.name,
        machine_id: node.machine_id,
        route_policy_id: node.route_policy_id ?? "",
        admin_route_policy_id: node.admin_route_policy_id ?? "",
        member_route_policy_id: node.member_route_policy_id ?? "",
        protocol: node.protocol === "hysteria2" ? "hysteria" : node.protocol,
        listen_ip: node.listen_ip,
        server_port: String(node.server_port),
        kernel_type: node.kernel_type,
        config: prepareHysteriaTLS(node.protocol, node.protocol === "hysteria2" ? { ...node.config, version: 2 } : node.config ?? {}),
        endpoints: (node.endpoints ?? []).map((endpoint) => ({ ...endpoint, access_scope: endpoint.access_scope || "default" })),
      };
      initialized.current = true; setForm(next); setInitial(JSON.stringify(next));
    }).catch((reason) => {
      if (controller.signal.aborted) return;
      if (reason instanceof ApiError && reason.status === 404) setMissing(true);
      else setLoadError(requestErrorMessage(reason, "节点加载失败"));
    }).finally(() => { if (!controller.signal.aborted) setNodeLoading(false); });
    return () => controller.abort();
  }, [attempt, copyID, copying, id]);

  useEffect(() => {
    if (editing || copying || initialized.current || relatedLoading || relatedError) return;
    const stored = new Map(nodeDefaults.map((item) => [item.key, String(item.value ?? "")]));
    const next = { ...emptyForm, config: structuredClone(emptyForm.config), endpoints: [], machine_id: machines[0]?.id || "", kernel_type: stored.get("default_kernel") || "xray", listen_ip: stored.get("listen_ip") || "0.0.0.0" };
    initialized.current = true; setForm(next); setInitial(JSON.stringify(next));
  }, [copying, editing, machines, nodeDefaults, relatedError, relatedLoading]);

  const selectedMachine = useMemo(
    () => machines.find((item) => item.id === form.machine_id),
    [machines, form.machine_id],
  );
  function update<K extends keyof NodeForm>(key: K, value: NodeForm[K]) {
    setForm((current) => ({ ...current, [key]: value }));
    setErrors((current) => ({ ...current, [key]: "" }));
  }

  function normalizeErrors(fields: Record<string, string>) {
    return Object.fromEntries(Object.entries(fields).map(([key, message]) => [
      key.startsWith("config.") ? "config" : key.replace(/^endpoints[.\[](\d+)\]?\.([a-z_]+)$/, "endpoint_$1_$2"), message,
    ]));
  }

  function focusError(next: Record<string, string>) {
    requestAnimationFrame(() => {
      const key = Object.keys(next)[0];
      const target = document.getElementById(key === "config" ? "node-config-error" : key)
        ?? document.getElementById("node-request-error");
      target?.focus();
    });
  }

  function validate() {
    const next: Record<string, string> = {};
    if (!form.name.trim()) next.name = "请填写节点名称";
    if (!form.machine_id) next.machine_id = "请选择服务器";
    const port = Number(form.server_port);
    if (!Number.isInteger(port) || port < 1 || port > 65535)
      next.server_port = "端口必须是 1 到 65535 之间的整数";
    const configError = validateNodeSettings(form.protocol, form.kernel_type, form.config);
    if (configError) next.config = configError;
    form.endpoints.forEach((endpoint, index) => {
      if (!endpoint.name.trim()) next[`endpoint_${index}_name`] = "请填写入口名称";
      if (!endpoint.host.trim()) next[`endpoint_${index}_host`] = "请填写入口地址";
      if (!Number.isInteger(Number(endpoint.port)) || Number(endpoint.port) < 1 || Number(endpoint.port) > 65535) next[`endpoint_${index}_port`] = "端口必须是 1 到 65535 之间的整数";
    });
    setErrors(next);
    if (Object.keys(next).length) focusError(next);
    return Object.keys(next).length === 0;
  }

  async function submit(event: FormEvent, publish: boolean) {
    event.preventDefault();
    if (savingRef.current || !initialized.current || relatedLoading || relatedError || !validate()) return;
    if (savedID.current && configVersion.current === undefined) { setRequestError("服务未返回配置版本，请重新加载节点后再保存。"); return; }
    savingRef.current = true;
    setSaving(publish ? "publish" : "save");
    setRequestError("");
    setRequestFailure(null);
    const payload = {
      name: form.name.trim(),
      machine_id: form.machine_id,
      route_policy_id: form.route_policy_id || (editing ? "" : null),
      admin_route_policy_id: form.admin_route_policy_id || (editing ? "" : null),
      member_route_policy_id: form.member_route_policy_id || (editing ? "" : null),
      protocol: form.protocol,
      listen_ip: form.listen_ip.trim() || "0.0.0.0",
      server_port: Number(form.server_port),
      kernel_type: form.kernel_type,
      config: prepareHysteriaTLS(form.protocol, form.config),
      endpoints: form.endpoints.map((endpoint, index) => ({ ...endpoint, name: endpoint.name.trim(), host: endpoint.host.trim(), port: Number(endpoint.port), sort_order: index })),
    };
    let configurationSaved = false;
    try {
      if (!(publish && pendingPublish && !dirty && savedID.current)) {
        const node = savedID.current
        ? configVersion.current !== undefined
          ? await api.patch<Node>(`/nodes/${savedID.current}`, payload, { configVersion: configVersion.current })
          : await api.patch<Node>(`/nodes/${savedID.current}`, payload)
        : await api.post<Node>("/nodes", payload, { idempotencyKey: intent.current(payload) });
        savedID.current = node.id;
        configVersion.current = node.config_version;
        setInitial(JSON.stringify(form));
      }
      configurationSaved = true;
      if (publish) {
        const key = pendingPublish && !dirty && pendingPublishIntent.current ? pendingPublishIntent.current : publishIntent.current({ id: savedID.current, config_version: configVersion.current });
        pendingPublishIntent.current = key;
        await api.post(`/nodes/${savedID.current}/publish`, undefined, { idempotencyKey: key, configVersion: configVersion.current });
      }
      setPendingPublish(false);
      pendingPublishIntent.current = "";
      toast(publish ? "配置已提交，等待 Agent 应用" : "节点草稿已保存，尚未发布");
      bypass(() => navigate(returnTo, { replace: true }));
    } catch (reason) {
      setRequestFailure(reason);
      if (configurationSaved && publish) {
        setPendingPublish(true);
        setRequestError(`配置已保存，发布失败：${requestErrorMessage(reason, "请重试发布")}`);
        if (!editing && savedID.current) {
          const query = new URLSearchParams({ returnTo });
          bypass(() => navigate(`/nodes/${encodeURIComponent(savedID.current)}/edit?${query}`, { replace: true, state: { nodePublishFailed: true, nodePublishIntent: pendingPublishIntent.current } }));
        }
      } else {
        setRequestError(requestErrorMessage(reason, "保存失败，请检查连接后重试"));
        const next = reason instanceof ApiError ? normalizeErrors(reason.fields) : {};
        setErrors(next);
        focusError(next);
      }
    } finally {
      savingRef.current = false; setSaving("");
    }
  }

  if (!nodeLoading && !relatedLoading && (loadError || relatedError || missing)) return <div className="page resource-editor-page"><PageHeader title={missing ? "节点不存在" : "节点加载失败"} /><EmptyState title={missing ? "记录可能已被删除" : "无法加载节点或关联配置"} description={loadError || relatedError || "请返回列表确认。"} action={missing ? <Link className="button button--secondary" to={returnTo}>返回节点列表</Link> : <Button onClick={() => { setAttempt((value) => value + 1); void reloadMachines(); void reloadRoutes(); void reloadDefaults(); }}>重新加载</Button>} /></div>;

  if (nodeLoading || relatedLoading)
    return (
      <div className="page resource-page operations-page admin-node-editor">
         <PageHeader title={editing ? "编辑节点" : copying ? "复制节点" : "添加节点"} />
        <TableSkeleton columns={2} rows={7} />
      </div>
    );
  return (
    <div className="page page--editor resource-page operations-page admin-node-editor">
      <div className="back-row">
        <Link to={returnTo}>
          <ArrowLeft size={16} />
          返回节点列表
        </Link>
      </div>
      <PageHeader
        title={editing ? "编辑节点" : copying ? "复制节点" : "添加节点"}
        actions={id && <Link className="button button--secondary" to={`/nodes/${encodeURIComponent(id)}/history?${new URLSearchParams({ returnTo })}`}>版本历史</Link>}
        description={
          editing
            ? "保存草稿保留当前运行版本；发布后等待 Agent 确认应用。"
            : copying
              ? "已复制原节点配置，请修改名称和监听端口后保存。"
            : "创建节点配置并绑定到一台 Corade 服务器。"
        }
      />
      <div className="editor-layout node-editor-layout"><nav className="editor-section-nav" aria-label="节点编辑章节">
        <button type="button" aria-current={activeSection === "node-endpoints" ? "location" : undefined} onClick={() => { setActiveSection("node-endpoints"); document.getElementById("node-endpoints")?.scrollIntoView({ block: "start", behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "instant" : "smooth" }); }}>节点入口</button>
        <button type="button" aria-current={activeSection === "node-basics" ? "location" : undefined} onClick={() => { setActiveSection("node-basics"); document.getElementById("node-basics")?.scrollIntoView({ block: "start", behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "instant" : "smooth" }); }}>基础信息</button>
        <button type="button" aria-current={activeSection === "node-listener" ? "location" : undefined} onClick={() => { setActiveSection("node-listener"); document.getElementById("node-listener")?.scrollIntoView({ block: "start", behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "instant" : "smooth" }); }}>协议与监听</button>
        <button type="button" aria-current={activeSection === "node-protocol" ? "location" : undefined} onClick={() => { setActiveSection("node-protocol"); document.getElementById("node-protocol")?.scrollIntoView({ block: "start", behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "instant" : "smooth" }); }}>协议配置</button>
      </nav>
      <form
        className="editor-form"
        aria-label={editing ? "编辑节点" : copying ? "复制节点" : "添加节点"}
        aria-busy={Boolean(saving)}
        noValidate
        onSubmit={(event) => void submit(event, false)}
      >
        {requestError && <div id="node-request-error" className="form-alert" role="alert" tabIndex={-1}>{requestError}<ErrorDiagnostic error={requestFailure} /></div>}
        {((savedID.current && initialized.current && configVersion.current === undefined) || (requestFailure instanceof ApiError && ["CONFIG_VERSION_CONFLICT", "EDIT_VERSION_CONFLICT"].includes(requestFailure.code))) && <VersionConflict version={configVersion.current} missing={configVersion.current === undefined} onReload={() => { setRequestError(""); setRequestFailure(null); setErrors({}); setAttempt((value) => value + 1); }} />}
        <fieldset disabled={Boolean(saving)} className="editor-fields">
        <section className="form-section" id="node-endpoints">
          <div className="form-section__heading">
            <div>
              <h2>节点入口</h2>
              <p>维护订阅中展示的入口名称、连接地址、端口和权限范围；不会创建额外 Corade 入站。</p>
            </div>
            <Button type="button" onClick={() => update("endpoints", [...form.endpoints, { name: form.name || "新入口", host: selectedMachine?.host || "", port: Number(form.server_port) || 443, status: "active", access_scope: "default", sort_order: form.endpoints.length }])}><Plus size={15} />添加入口</Button>
          </div>
          {form.endpoints.length === 0 ? <div className="endpoint-empty"><p>未配置入口时，订阅将使用服务器地址和节点监听端口（默认权限）。</p><Button type="button" onClick={() => update("endpoints", [{ name: form.name || "新入口", host: selectedMachine?.host || "", port: Number(form.server_port) || 443, status: "active", access_scope: "default", sort_order: 0 }])}><Plus size={15} />添加第一个入口</Button></div> : <div className="endpoint-table table-scroll"><table><thead><tr><th>入口名称</th><th>地址</th><th>端口</th><th>权限组</th><th>状态</th><th className="col-actions">操作</th></tr></thead><tbody>{form.endpoints.map((endpoint, index) => <tr key={endpoint.id ?? index}><td data-label="入口名称"><input id={`endpoint_${index}_name`} aria-invalid={Boolean(errors[`endpoint_${index}_name`])} aria-describedby={errors[`endpoint_${index}_name`] ? `endpoint_${index}_name-error` : undefined} aria-label={`入口 ${index + 1} 名称`} value={endpoint.name} onChange={(event) => update("endpoints", form.endpoints.map((item, itemIndex) => itemIndex === index ? { ...item, name: event.target.value } : item))} />{errors[`endpoint_${index}_name`] && <span id={`endpoint_${index}_name-error`} className="field__error">{errors[`endpoint_${index}_name`]}</span>}</td><td data-label="地址"><input id={`endpoint_${index}_host`} aria-invalid={Boolean(errors[`endpoint_${index}_host`])} aria-describedby={errors[`endpoint_${index}_host`] ? `endpoint_${index}_host-error` : undefined} aria-label={`入口 ${index + 1} 地址`} className="mono" value={endpoint.host} onChange={(event) => update("endpoints", form.endpoints.map((item, itemIndex) => itemIndex === index ? { ...item, host: event.target.value } : item))} />{errors[`endpoint_${index}_host`] && <span id={`endpoint_${index}_host-error`} className="field__error">{errors[`endpoint_${index}_host`]}</span>}</td><td data-label="端口"><input id={`endpoint_${index}_port`} aria-invalid={Boolean(errors[`endpoint_${index}_port`])} aria-describedby={errors[`endpoint_${index}_port`] ? `endpoint_${index}_port-error` : undefined} aria-label={`入口 ${index + 1} 端口`} className="port-input mono" inputMode="numeric" type="number" min="1" max="65535" value={endpoint.port} onChange={(event) => update("endpoints", form.endpoints.map((item, itemIndex) => itemIndex === index ? { ...item, port: Number(event.target.value) } : item))} />{errors[`endpoint_${index}_port`] && <span id={`endpoint_${index}_port-error`} className="field__error">{errors[`endpoint_${index}_port`]}</span>}</td><td data-label="权限范围"><select id={`endpoint_${index}_access_scope`} aria-invalid={Boolean(errors[`endpoint_${index}_access_scope`])} aria-describedby={errors[`endpoint_${index}_access_scope`] ? `endpoint_${index}_access_scope-error` : undefined} aria-label={`入口 ${index + 1} 权限组`} value={endpoint.access_scope || "default"} onChange={(event) => update("endpoints", form.endpoints.map((item, itemIndex) => itemIndex === index ? { ...item, access_scope: event.target.value as NodeEndpoint["access_scope"] } : item))}><option value="default">默认</option><option value="admin">仅管理员</option></select>{errors[`endpoint_${index}_access_scope`] && <span id={`endpoint_${index}_access_scope-error`} className="field__error">{errors[`endpoint_${index}_access_scope`]}</span>}</td><td data-label="状态"><select aria-label={`入口 ${index + 1} 状态`} value={endpoint.status} onChange={(event) => update("endpoints", form.endpoints.map((item, itemIndex) => itemIndex === index ? { ...item, status: event.target.value as NodeEndpoint["status"] } : item))}><option value="active">启用</option><option value="disabled">停用</option></select></td><td data-label="操作" className="row-actions"><button type="button" className="icon-button" aria-label={`删除入口 ${endpoint.name}`} title="删除入口" onClick={() => update("endpoints", form.endpoints.filter((_, itemIndex) => itemIndex !== index))}><Trash2 size={16} /></button></td></tr>)}</tbody></table></div>}
        </section>
        <section className="form-section" id="node-basics">
          <div className="form-section__heading">
            <div>
              <h2>基础信息</h2>
              <p>用于在后台识别节点并确定运行位置。</p>
            </div>
            <Info size={18} />
          </div>
          <div className="form-grid">
            <Field label="节点名称" required error={errors.name}>
              <input
                id="name"
                value={form.name}
                onChange={(event) => update("name", event.target.value)}
                placeholder="例如：香港 VLESS 主入口"
                autoFocus
              />
            </Field>
            <Field
              label="所属服务器"
              required
              error={errors.machine_id}
              helper={
                selectedMachine
                  ? `服务器内核：${selectedMachine.kernel_type}`
                  : undefined
              }
            >
              <select
                id="machine_id"
                value={form.machine_id}
                onChange={(event) => {
                  update("machine_id", event.target.value);
                }}
              >
                <option value="">请选择服务器</option>
                {form.machine_id && !machines.some((item) => item.id === form.machine_id) && <option value={form.machine_id}>未找到服务器（{form.machine_id}）</option>}
                {machines.map((machine) => (
                  <option value={machine.id} key={machine.id}>
                    {machine.name} · {machine.region || "未设置区域"}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="路由策略" error={errors.route_policy_id} helper="留空时使用系统默认直连策略">
              <select
                id="route_policy_id"
                value={form.route_policy_id}
                onChange={(event) =>
                  update("route_policy_id", event.target.value)
                }
              >
                <option value="">不单独绑定</option>
                {form.route_policy_id && !routes.some((item) => item.id === form.route_policy_id) && <option value={form.route_policy_id}>未找到原绑定策略（{form.route_policy_id}）</option>}
                {routes.filter((route) => (route.scope ?? "default") === "default").map((route) => (
                  <option value={route.id} key={route.id}>
                    {route.name} · rev {route.current_revision}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="管理员策略" error={errors.admin_route_policy_id} helper="admin 订阅账号使用；留空回退默认策略">
              <select id="admin_route_policy_id" value={form.admin_route_policy_id} onChange={(event) => update("admin_route_policy_id", event.target.value)}>
                <option value="">跟随默认策略</option>
                {form.admin_route_policy_id && !routes.some((item) => item.id === form.admin_route_policy_id) && <option value={form.admin_route_policy_id}>未找到原绑定策略（{form.admin_route_policy_id}）</option>}
                {routes.filter((route) => route.scope === "admin").map((route) => <option value={route.id} key={route.id}>{route.name} · rev {route.current_revision}</option>)}
              </select>
            </Field>
            <Field label="用户/朋友策略" error={errors.member_route_policy_id} helper="user 与 friend 共用；留空回退默认策略">
              <select id="member_route_policy_id" value={form.member_route_policy_id} onChange={(event) => update("member_route_policy_id", event.target.value)}>
                <option value="">跟随默认策略</option>
                {form.member_route_policy_id && !routes.some((item) => item.id === form.member_route_policy_id) && <option value={form.member_route_policy_id}>未找到原绑定策略（{form.member_route_policy_id}）</option>}
                {routes.filter((route) => route.scope === "member").map((route) => <option value={route.id} key={route.id}>{route.name} · rev {route.current_revision}</option>)}
              </select>
            </Field>
          </div>
        </section>
        <section className="form-section" id="node-listener">
          <div className="form-section__heading">
            <div>
              <h2>协议与监听</h2>
              <p>定义 Corade 创建的入站服务及本机监听地址。</p>
            </div>
          </div>
          <div className="form-grid form-grid--three">
            <Field label="内核" required error={errors.kernel_type}>
              <select
                id="kernel_type"
                value={form.kernel_type}
                onChange={(event) => update("kernel_type", event.target.value)}
              >
                <option value="xray">Xray</option>
                <option value="singbox">sing-box</option>
              </select>
            </Field>
            <Field label="监听地址" required error={errors.listen_ip}>
              <input
                id="listen_ip"
                value={form.listen_ip}
                onChange={(event) => update("listen_ip", event.target.value)}
                className="mono"
              />
            </Field>
            <Field label="端口" required error={errors.server_port}>
              <input
                id="server_port"
                className="port-input mono"
                inputMode="numeric"
                type="number"
                min="1"
                max="65535"
                value={form.server_port}
                onChange={(event) => update("server_port", event.target.value)}
              />
            </Field>
          </div>
          <div className="inline-notice">
            {protocolSupported(form.protocol, form.kernel_type) ? <CheckCircle2 size={16} /> : <Info size={16} />}
            <span>
              {protocolSupported(form.protocol, form.kernel_type)
                ? "保存时会检查同一服务器上的端口冲突，当前节点类型支持所选内核。"
                : "当前节点类型不支持所选内核，请在下方重新选择节点类型。"}
            </span>
          </div>
        </section>
        <section className="form-section" id="node-protocol">
          <div className="form-section__heading">
            <div>
              <h2>节点类型与协议配置</h2>
              <p>选择节点类型后，通过表单维护传输、安全和协议参数。</p>
            </div>
            <SlidersHorizontal size={18} />
          </div>
          <NodeConfigForm
            protocol={form.protocol}
            kernel={form.kernel_type}
            settings={form.config}
            error={errors.config}
            errorID="node-config-error"
            onProtocolChange={(value) => update("protocol", value)}
            onSettingsChange={(value) => update("config", value)}
          />
        </section>
        </fieldset>
        <footer className="editor-actions">
          <span className="editor-status" role="status">{saving ? "正在提交…" : dirty ? "有未保存修改" : pendingPublish ? "配置已保存，等待重新发布" : "配置已加载"}</span>
          <div>
            <Button type="button" variant="ghost" disabled={Boolean(saving)} onClick={() => navigate(returnTo, { replace: true })}>取消</Button>
          </div>
          <div>
            <Button type="submit" loading={saving === "save"} disabled={Boolean(saving)}>
              <Save size={16} />
              保存草稿
            </Button>
            <Button
              type="button"
              variant="primary"
              loading={saving === "publish"}
              disabled={Boolean(saving)}
              onClick={(event) =>
                void submit(event as unknown as FormEvent, true)
              }
            >
              <Rocket size={16} />
              {pendingPublish && !dirty ? "重试发布" : "保存并发布"}
            </Button>
          </div>
        </footer>
      </form></div>
    </div>
  );
}
