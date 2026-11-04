import { useState } from "react";
import { Button, ConfirmDialog } from "./ui";

export function VersionConflict({ version, missing = false, onReload }: { version?: number; missing?: boolean; onReload: () => void }) {
  const [confirm, setConfirm] = useState(false);
  return <div className="form-alert" role="alert"><p>{missing ? "服务未返回配置版本，暂时无法安全保存。当前输入已保留。" : `服务端配置已被其他操作修改。本页基于版本 ${version ?? "未知"}，当前输入已保留。`}</p><p>请先在新标签页查看最新配置，确认差异后重新加载；不会自动覆盖任何一方的配置。</p><div className="conflict-actions"><a className="button button--secondary" href={location.pathname + location.search} target="_blank" rel="noopener noreferrer">查看最新配置</a><Button type="button" onClick={() => setConfirm(true)}>放弃本页修改并重新加载</Button></div><ConfirmDialog open={confirm} title="放弃本页修改？" description="将读取服务端最新配置，本页未保存的输入会被替换。" confirmLabel="重新加载" onClose={() => setConfirm(false)} onConfirm={() => { setConfirm(false); onReload(); }} /></div>;
}
