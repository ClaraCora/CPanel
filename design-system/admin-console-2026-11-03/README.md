# CPanel 后台改造交付证据

日期：2026-11-03。代码分支：`ui/admin-console-2026-11-03`。

本目录保留代表性桌面、手机、明暗及完整编辑截图和主要验证记录。完整 13 页面 × 3 尺寸 × 2 主题、七类编辑页及异常状态截图位于项目本地 `.impeccable/review/admin-2026-11-03/round2/`，其余大批截图不纳入 Git。

实际改造与回退说明见 [改造记录](../admin-console-migration.md)。本地开发预览为 [服务器管理](http://127.0.0.1:5195/machines?demo=1)，演示模式登录表单已填写合成凭据，点击“登录后台”后查看；演示写操作不会进入真实数据库。开发服务关闭后，在 `web` 目录运行 `npm run dev -- --host 127.0.0.1 --port 5195` 恢复。

本目录图片使用合成数据，不包含真实管理员密码、生产令牌、证书或私钥。界面没有新增产品图片，截图仅用于开发验收。

| 文件 | 内容 |
|---|---|
| `machines-1440-light.png` / `machines-1440-dark.png` | 桌面服务器卡片 |
| `machines-390-light.png` / `machines-390-dark.png` | 手机服务器与完整区域筛选 |
| `machine-editor-1440-light.png` | 服务器独立编辑页；长截图中的操作栏为粘性定位 |
| `node-editor-bottom-390-light.png` | 节点手机编辑底部，最后字段与保存操作 |
| `settings-1440-light.png` | 设置分区与保存 |
| `ip-detail-390-light.png` | 手机全屏只读 IP 详情 |
| `finish-review.md` / `finish-verdict.md` | 首轮三项修正及最终 scoped ship 复核 |
| `review-fixes.json` | 修正后的实际尺寸、焦点及输入保留检查 |
| `workflows.json` | 浏览器未保存保护、过滤恢复、遮蔽与设置草稿 |
| `real-workflows.json` | 隔离后端的七类资源新增、修改、刷新与数据回读 |
| `portal-session.json` / `session-expiry.json` | 真实只读代入与挂载中的会话失效 |

真实测试使用独立 PostgreSQL 集群及本地后端。没有向生产 Agent 发送安装、升级、身份重置或 TG 消息，没有部署、推送和合并。
