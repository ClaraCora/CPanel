# 后台界面改造记录

实施依据：`C:\Users\ccios\Desktop\CPanel\CPanel界面重构实施方案.MD`。

## T00 基线

- 日期：2026-11-03；基线：`9a767f51ac1ac3e745d9a537e084cfe06e66a453`。
- 分支：`ui/admin-console-2026-11-03`；开始时工作区干净。
- 前端基线：7 个测试文件、21 项测试通过；类型检查通过。
- 基线截图：项目根目录 `.impeccable/review/baseline/`，包含 13 个后台页面、手机服务器页、登录页与门户页面。截图使用开发演示数据，不读取生产账号。
- 编辑现状：节点已有独立页面；服务器、账号、套餐、权限组、路由、出站使用通用右侧抽屉。
- 只读抽屉：安装命令、访问 IP、审计详情。

## 字段与动作兼容清单

| 资源 | 保留的编辑字段 | 保留的动作及约束 |
|---|---|---|
| 服务器 | name、region、host、labels、notes | 安装、升级全部任务状态、身份重置、启停、关联删除限制、15 秒刷新 |
| 节点 | name、machine_id、三类 route_policy_id、protocol、listen_ip、server_port、kernel_type、config、endpoints | 草稿、发布、复制、停用、恢复草稿、删除；协议/TLS/证书校验；应用版本由 Agent 回报 |
| 权限组 | name、node_ids、notes | 节点多选、启停、引用删除限制；订阅访问范围而非后台 RBAC |
| 套餐 | name、access_group_id、traffic_limit_bytes、speed_limit_mbps、device_limit、reset_strategy、default_valid_days、notes | 1024 进制流量换算、自然月/不重置、启停与引用限制 |
| 账号 | role、uuid、name、email、plan_id、expires_at、notes、portal_login、portal_password | 管理员保护、快捷创建、复制订阅、只读门户代入、暂停与删除；密码空值保留 |
| 路由 | name、scope、default_outbound_tag、rules、notes | 规则增删排序、匹配类型/动作、默认出口限制、启停与关联删除限制 |
| 出站 | name、tag、protocol、settings、proxy_tag、kernel_support | 原有协议表单、链式代理限制、启停与引用删除限制；不增加探测 |
| 设置/订阅 | 全部现有分区键、类型、默认值、敏感值标志 | 分区保存、切换保留草稿、敏感值留空保留、密码验证、TG 测试；访问日志真实分页 |

## 页面与兼容边界

保留 `/`、`/machines`、`/nodes`、`/users`、`/plans`、`/access-groups`、`/subscription`、`/access-ips`、`/routes`、`/outbounds`、`/history`、`/settings`、`/audit`。

仅节点具有单条读取接口，其他六类编辑从现有完整列表按 ID 加载。保留逻辑 API 到 `/ca/ht` 的映射、CSRF 与 Cookie、认证后的应用加载、受保护静态资源分块、登录与 `/edu` 样式及权限边界。

后续任务的提交、验收证据和回退记录与实施方案中的 T00–T16 同步维护。
