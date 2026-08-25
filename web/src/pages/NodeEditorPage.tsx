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
  Field,
  PageHeader,
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
  const [searchParams] = useSearchParams();
  const copyID = searchParams.get("copy");
  const editing = Boolean(id);
  const copying = !editing && Boolean(copyID);
  const [, navigate] = useLocation();
  const toast = useToast();
  const { data: machines, loading: machinesLoading } = useResource<Machine[]>(
    "/machines",
    [],
  );
  const { data: routes } = useResource<RoutePolicy[]>("/route-policies", []);
  const { data: nodeDefaults } = useResource<Setting[]>("/settings/node_defaults", []);
  const defaultsApplied = useRef(false);
  const [form, setForm] = useState<NodeForm>(emptyForm);
  const [nodeLoading, setNodeLoading] = useState(editing || copying);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [requestError, setRequestError] = useState("");
  const [saving, setSaving] = useState<"save" | "publish" | "">("");

  useEffect(() => {
    const sourceID = id || copyID;
    if (!sourceID) return;
    setNodeLoading(true);
    api
      .get<Node>(`/nodes/${sourceID}`)
      .then((node) =>
        setForm({
          name: copying ? `${node.name} 副本` : node.name,
          machine_id: node.machine_id,
          route_policy_id: node.route_policy_id ?? "",
          admin_route_policy_id: node.admin_route_policy_id ?? "",
          member_route_policy_id: node.member_route_policy_id ?? "",
          protocol: node.protocol,
          listen_ip: node.listen_ip,
          server_port: String(node.server_port),
          kernel_type: node.kernel_type,
          config: node.config ?? {},
          endpoints: (node.endpoints ?? []).map((endpoint) => ({ ...endpoint, access_scope: endpoint.access_scope || "default" })),
        }),
      )
      .catch((reason) =>
        setRequestError(
          reason instanceof Error ? reason.message : "节点加载失败",
        ),
      )
      .finally(() => setNodeLoading(false));
  }, [copyID, copying, id]);

  useEffect(() => {
    if (!editing && !copying && !defaultsApplied.current && nodeDefaults.length) {
      const stored = new Map(nodeDefaults.map((item) => [item.key, String(item.value ?? "")]));
      setForm((value) => ({
        ...value,
        kernel_type: stored.get("default_kernel") || "xray",
        listen_ip: stored.get("listen_ip") || "0.0.0.0",
      }));
      defaultsApplied.current = true;
    }
  }, [copying, editing, nodeDefaults]);

  useEffect(() => {
    if (!editing && !copying && !form.machine_id && machines.length)
      setForm((value) => ({ ...value, machine_id: machines[0].id }));
  }, [copying, editing, form.machine_id, machines]);

  const selectedMachine = useMemo(
    () => machines.find((item) => item.id === form.machine_id),
    [machines, form.machine_id],
  );
  function update<K extends keyof NodeForm>(key: K, value: NodeForm[K]) {
    setForm((current) => ({ ...current, [key]: value }));
    setErrors((current) => ({ ...current, [key]: "" }));
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
    if (Object.keys(next).length)
      document.getElementById(Object.keys(next)[0])?.focus();
    return Object.keys(next).length === 0;
  }

  async function submit(event: FormEvent, publish: boolean) {
    event.preventDefault();
    if (!validate()) return;
    setSaving(publish ? "publish" : "save");
    setRequestError("");
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
      config: form.config,
      endpoints: form.endpoints.map((endpoint, index) => ({ ...endpoint, name: endpoint.name.trim(), host: endpoint.host.trim(), port: Number(endpoint.port), sort_order: index })),
    };
    try {
      const node = editing
        ? await api.patch<Node>(`/nodes/${id}`, payload)
        : await api.post<Node>("/nodes", payload);
      if (publish) await api.post(`/nodes/${node.id}/publish`);
      toast(publish ? "节点配置已保存并发布" : "节点配置已保存");
      navigate("/nodes");
    } catch (reason) {
      if (reason instanceof ApiError) {
        setRequestError(reason.message);
        setErrors(reason.fields);
      } else setRequestError("保存失败，请检查连接后重试");
    } finally {
      setSaving("");
    }
  }

  if (nodeLoading || machinesLoading)
    return (
      <div className="page">
         <PageHeader title={editing ? "编辑节点" : copying ? "复制节点" : "添加节点"} />
        <TableSkeleton columns={2} rows={7} />
      </div>
    );
  return (
    <div className="page page--editor">
      <div className="back-row">
        <Link to="/nodes">
          <ArrowLeft size={16} />
          返回节点列表
        </Link>
      </div>
      <PageHeader
        title={editing ? "编辑节点" : copying ? "复制节点" : "添加节点"}
        description={
          editing
            ? "保存后节点将变为待发布，当前运行版本不会立即改变。"
            : copying
              ? "已复制原节点配置，请修改名称和监听端口后保存。"
            : "创建节点配置并绑定到一台 Corade 服务器。"
        }
      />
      {requestError && (
        <div className="form-alert" role="alert">
          {requestError}
        </div>
      )}
      <form
        className="editor-form"
        onSubmit={(event) => void submit(event, false)}
      >
        <section className="form-section">
          <div className="form-section__heading">
            <div>
              <h2>节点入口</h2>
              <p>维护订阅中展示的入口名称、连接地址、端口和权限范围；不会创建额外 Corade 入站。</p>
            </div>
            <Button type="button" onClick={() => update("endpoints", [...form.endpoints, { name: form.name || "新入口", host: selectedMachine?.host || "", port: Number(form.server_port) || 443, status: "active", access_scope: "default", sort_order: form.endpoints.length }])}><Plus size={15} />添加入口</Button>
          </div>
          {form.endpoints.length === 0 ? <div className="endpoint-empty"><p>未配置入口时，订阅将使用服务器地址和节点监听端口（默认权限）。</p><Button type="button" onClick={() => update("endpoints", [{ name: form.name || "新入口", host: selectedMachine?.host || "", port: Number(form.server_port) || 443, status: "active", access_scope: "default", sort_order: 0 }])}><Plus size={15} />添加第一个入口</Button></div> : <div className="endpoint-table table-scroll"><table><thead><tr><th>入口名称</th><th>地址</th><th>端口</th><th>权限组</th><th>状态</th><th className="col-actions">操作</th></tr></thead><tbody>{form.endpoints.map((endpoint, index) => <tr key={endpoint.id ?? index}><td><input aria-label={`入口 ${index + 1} 名称`} value={endpoint.name} onChange={(event) => update("endpoints", form.endpoints.map((item, itemIndex) => itemIndex === index ? { ...item, name: event.target.value } : item))} />{errors[`endpoint_${index}_name`] && <span className="field__error">{errors[`endpoint_${index}_name`]}</span>}</td><td><input aria-label={`入口 ${index + 1} 地址`} className="mono" value={endpoint.host} onChange={(event) => update("endpoints", form.endpoints.map((item, itemIndex) => itemIndex === index ? { ...item, host: event.target.value } : item))} />{errors[`endpoint_${index}_host`] && <span className="field__error">{errors[`endpoint_${index}_host`]}</span>}</td><td><input aria-label={`入口 ${index + 1} 端口`} type="number" min="1" max="65535" value={endpoint.port} onChange={(event) => update("endpoints", form.endpoints.map((item, itemIndex) => itemIndex === index ? { ...item, port: Number(event.target.value) } : item))} />{errors[`endpoint_${index}_port`] && <span className="field__error">{errors[`endpoint_${index}_port`]}</span>}</td><td><select aria-label={`入口 ${index + 1} 权限组`} value={endpoint.access_scope || "default"} onChange={(event) => update("endpoints", form.endpoints.map((item, itemIndex) => itemIndex === index ? { ...item, access_scope: event.target.value as NodeEndpoint["access_scope"] } : item))}><option value="default">默认</option><option value="admin">仅管理员</option></select>{errors[`endpoint_${index}_access_scope`] && <span className="field__error">{errors[`endpoint_${index}_access_scope`]}</span>}</td><td><select aria-label={`入口 ${index + 1} 状态`} value={endpoint.status} onChange={(event) => update("endpoints", form.endpoints.map((item, itemIndex) => itemIndex === index ? { ...item, status: event.target.value as NodeEndpoint["status"] } : item))}><option value="active">启用</option><option value="disabled">停用</option></select></td><td className="row-actions"><button type="button" className="icon-button" aria-label={`删除入口 ${endpoint.name}`} title="删除入口" onClick={() => update("endpoints", form.endpoints.filter((_, itemIndex) => itemIndex !== index))}><Trash2 size={16} /></button></td></tr>)}</tbody></table></div>}
        </section>
        <section className="form-section">
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
                {machines.map((machine) => (
                  <option value={machine.id} key={machine.id}>
                    {machine.name} · {machine.region || "未设置区域"}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="路由策略" helper="留空时使用系统默认直连策略">
              <select
                value={form.route_policy_id}
                onChange={(event) =>
                  update("route_policy_id", event.target.value)
                }
              >
                <option value="">不单独绑定</option>
                {routes.filter((route) => (route.scope ?? "default") === "default").map((route) => (
                  <option value={route.id} key={route.id}>
                    {route.name} · rev {route.current_revision}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="管理员策略" helper="admin 订阅账号使用；留空回退默认策略">
              <select value={form.admin_route_policy_id} onChange={(event) => update("admin_route_policy_id", event.target.value)}>
                <option value="">跟随默认策略</option>
                {routes.filter((route) => route.scope === "admin").map((route) => <option value={route.id} key={route.id}>{route.name} · rev {route.current_revision}</option>)}
              </select>
            </Field>
            <Field label="用户/朋友策略" helper="user 与 friend 共用；留空回退默认策略">
              <select value={form.member_route_policy_id} onChange={(event) => update("member_route_policy_id", event.target.value)}>
                <option value="">跟随默认策略</option>
                {routes.filter((route) => route.scope === "member").map((route) => <option value={route.id} key={route.id}>{route.name} · rev {route.current_revision}</option>)}
              </select>
            </Field>
          </div>
        </section>
        <section className="form-section">
          <div className="form-section__heading">
            <div>
              <h2>协议与监听</h2>
              <p>定义 Corade 创建的入站服务及本机监听地址。</p>
            </div>
          </div>
          <div className="form-grid form-grid--three">
            <Field label="内核" required>
              <select
                value={form.kernel_type}
                onChange={(event) => update("kernel_type", event.target.value)}
              >
                <option value="xray">Xray</option>
                <option value="singbox">sing-box</option>
              </select>
            </Field>
            <Field label="监听地址" required>
              <input
                value={form.listen_ip}
                onChange={(event) => update("listen_ip", event.target.value)}
                className="mono"
              />
            </Field>
            <Field label="端口" required error={errors.server_port}>
              <input
                id="server_port"
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
        <section className="form-section">
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
            onProtocolChange={(value) => update("protocol", value)}
            onSettingsChange={(value) => update("config", value)}
          />
        </section>
        <footer className="editor-actions">
          <div>
            <Link className="button button--ghost" to="/nodes">
              取消
            </Link>
          </div>
          <div>
            <Button type="submit" loading={saving === "save"}>
              <Save size={16} />
              保存草稿
            </Button>
            <Button
              type="button"
              variant="primary"
              loading={saving === "publish"}
              onClick={(event) =>
                void submit(event as unknown as FormEvent, true)
              }
            >
              <Rocket size={16} />
              保存并发布
            </Button>
          </div>
        </footer>
      </form>
    </div>
  );
}
