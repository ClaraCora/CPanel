# 节点配置应用确认

普通 `node.telemetry` 只记录运行采样及流量，不再推进 `applied_revision`。配置已经下载不等于内核已经应用。

## 事件

在原有认证、加密及幂等遥测传输中，新增 `node.application` 事件；无需新的公开入口或鉴权方式。

```json
{
  "type": "node.application",
  "node_id": 1042,
  "occurred_at": "2026-11-04T06:00:00Z",
  "data": {
    "revision": 12,
    "status": "failed",
    "stage": "kernel",
    "error": "reload routing: ..."
  }
}
```

- `received`：取得该版本配置；尚未证明转换、用户加载或内核启动成功。
- `applied`：内核成功启动或完成全部配置重载。相同配置的新版本可在确认运行内容一致后确认，无需重启。
- `failed`：转换、用户加载、运行校验、证书或内核应用失败。保留旧的已应用版本、旧运行配置及重试基线。
- `stage` 当前为 `fetch`、`convert`、`users`、`validate`、`certificate`、`kernel`。

节点没有授权用户时，Agent 按现有规则不启动内核，配置保持已接收；有用户并成功启动后才确认应用。失败重试不因 WebSocket 持续连接而停止。

Agent 在响应未知时保留原事件内容、时间及幂等键，随之后的遥测重试。面板只接受本机已发布节点、正数且不超过当前发布版本的结果；旧结果不能清除新尝试的失败。

## 后台字段

节点增加 `received_revision`、`attempted_revision`、`apply_status`、`apply_stage`；原有 `applied_revision` 与 `last_error` 继续使用。应用失败不将节点生命周期改成草稿或停用。

## 滚动升级

建议先升级面板，再升级 Agent。旧 Agent 无显式应用结果时，新面板会显示尚未确认，不能把普通心跳当成成功。

迁移 0031 将以前由普通遥测产生的 `applied_revision` 移入 `received_revision`，并将未经确认的已应用版本重置为 0。原值仍保留在接收版本中；升级后的 Agent 在真正启动或重载成功后重新确认。

新 Agent 对旧面板仍可发送原有遥测，其中 revision 已改成真正成功应用的版本；旧面板忽略新事件类型，因此原接口继续可用。新字段为增量迁移，回退旧二进制无需删列，但回退期间旧面板仍采用旧的遥测确认语义。

## 回归证据

- Store/PostgreSQL：普通遥测不确认、失败保留旧版本、显式成功确认、延迟旧结果不覆盖新失败、未来版本被忽略。
- Corade：配置转换/用户加载/运行校验/内核启动失败；重载失败恢复旧基线并允许重试；无用户时不虚报应用；响应丢失保留相同幂等键和请求体。
- sing-box 实际内核：既有出站地址及默认出口真正更新、部分替换失败恢复旧出口、链式出站换上游后可移除旧上游。
