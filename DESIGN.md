---
name: CPanel Administrator Console
description: Readable server operations with top navigation and complete editing pages.
colors:
  light-surface: "#f5f7fa"
  light-panel: "#fff"
  light-panel-strong: "#f1f5f9"
  light-foreground: "#1f2937"
  light-secondary: "#475569"
  light-border: "#e2e8f0"
  light-border-strong: "#94a3b8"
  light-primary: "#2563eb"
  light-primary-hover: "#1d4ed8"
  light-accent: "#1d4ed8"
  light-active: "#eff6ff"
  light-hover: "#f8fafc"
  light-success: "#166534"
  light-warning: "#92400e"
  light-danger: "#b91c1c"
  light-info: "#1d4ed8"
  light-success-bg: "#ecfdf3"
  light-warning-bg: "#fffbeb"
  light-danger-bg: "#fef2f2"
  light-info-bg: "#eff6ff"
  light-focus: "#2563eb"
  dark-surface: "#10151f"
  dark-panel: "#182130"
  dark-panel-strong: "#202c3d"
  dark-foreground: "#e5eaf2"
  dark-secondary: "#a7b4c8"
  dark-border: "#303b4c"
  dark-border-strong: "#64748b"
  dark-accent: "#93c5fd"
  dark-active: "#20375a"
  dark-hover: "#202c3d"
  dark-success: "#86efac"
  dark-warning: "#fcd34d"
  dark-danger: "#fca5a5"
  dark-info: "#93c5fd"
  dark-success-bg: "#19372c"
  dark-warning-bg: "#3e321c"
  dark-danger-bg: "#41282d"
  dark-info-bg: "#20375a"
  dark-focus: "#93c5fd"
  command-text: "#ffffff"
typography:
  title:
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei UI", sans-serif'
    fontSize: "26px"
    fontWeight: 650
    lineHeight: 1.35
    letterSpacing: "0"
  body:
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei UI", sans-serif'
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.6
    letterSpacing: "0"
  technical:
    fontFamily: '"Cascadia Mono", Consolas, monospace'
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.8
    letterSpacing: "0"
rounded:
  control: "8px"
  status: "5px"
  chip: "4px"
spacing:
  xs: "4px"
  sm: "8px"
  control-gap: "12px"
  md: "16px"
  lg: "24px"
  xl: "32px"
components:
  button-primary:
    backgroundColor: "{colors.light-primary}"
    textColor: "{colors.command-text}"
    rounded: "{rounded.control}"
    padding: "9px 15px"
    height: "40px"
  button-primary-hover:
    backgroundColor: "{colors.light-primary-hover}"
  panel-light:
    backgroundColor: "{colors.light-panel}"
    textColor: "{colors.light-foreground}"
    rounded: "{rounded.control}"
  panel-dark:
    backgroundColor: "{colors.dark-panel}"
    textColor: "{colors.dark-foreground}"
    rounded: "{rounded.control}"
---
# CPanel 设计规范

## Overview

**Creative North Star: "Readable operations, complete editing."**

本文件记录 2026-11-03 已实施的管理员后台设计。权威为用户的《CPanel界面重构实施方案.MD》和 `.impeccable/surfaces/admin-console.md`；旧的后台玻璃、侧栏及编辑抽屉规则已被替代。正文及示例描述现有实现，不添加业务能力。

后台使用顶部导航、浅色默认、蓝色主操作、完整深色主题和独立编辑页面。登录页及 `/edu` 保留既有外观和行为；公共页面保留 `web/src/portal.css` 基础，并由 `web/src/public-surfaces.css` 维护合并规则，不继承本轮后台色板。现有图表、协议、权限及请求契约以业务代码为准。

**Key Characteristics:**

- 64px 顶部导航与浅色默认。
- 七类资源独立编辑页面。
- 完整深色主题与移动记录布局。
- 真实运行状态与可恢复的表单上下文。

## Colors

规范 token 来自 `web/src/admin-console.css`。浅色使用 `#f5f7fa` 页面、白色面板、`#1f2937` 主文字、`#475569` 次文字和 `#e2e8f0` 分隔。深色使用 `#10151f` 页面、`#182130` 面板、`#e5eaf2` 主文字、`#a7b4c8` 次文字和 `#303b4c` 分隔。主按钮在两种主题均为 `#2563eb` 配白字；链接分别为 `#1d4ed8` 和 `#93c5fd`。

成功、待处理、危险和信息均使用各主题对应前景及底色，并同时显示文字。指标缺失为“—”，不能用零值表达。禁用控件降低透明度；危险操作不使用普通蓝色来弱化风险。

后台 token 限定在 `html[data-admin-theme]` 与 `.admin-console`。`#admin-overlays` 承接菜单和窗口。首次无偏好时为浅色，读写既有 `cpanel.theme`；后台卸载时恢复公共页面属性，门户继续独立使用 `cpanel.portal.theme`。公共 CSS 明确排除 `html[data-admin-theme]`。

## Typography

系统中文字体栈为 `-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei UI", sans-serif`，正文 14px / 1.6。标题 26px、650、1.35；分区标题 18px、600、1.5；小标题 15px、600。辅助数据多为 12–13px。正文和标题均不使用负字距。

IP、ID、端口、版本、数值和配置采用既有等宽类；代码和审计 JSON 使用 `"Cascadia Mono", Consolas, monospace`、13px / 1.8。数字使用 tabular-nums。手机输入字号 16px，避免编辑时缩放。不要把等宽字体推广到普通中文说明。

## Layout

主导航高 64px，顺序为总览、服务器、节点、账号与订阅、流量策略、系统；右侧为主题和管理员菜单。编辑页高亮所属资源。工作区最大 1440px，桌面左右 32px，低于 1200px 为 24px，低于 768px 为 16px。

服务器卡片在至少 1200px 为三列、768–1199px 两列、低于 768px 单列。导航低于 1024px 折为菜单。表格手机端显示字段标签、核心状态和操作；JSON、命令和复杂配置仅在组件内滚动。

服务器与节点列表采用局部紧凑密度：分类控件最小高度 44px，筛选区上下内边距 12px，桌面表格单元上下内边距 12px。服务器卡片内边距桌面为 20px、手机为 16px，指标分组上下外边距 16px。手机按钮保留 44px 最小触控高度。该密度仅用于这两类列表，不改变其他页面、编辑页或公共入口的间距规范。

普通编辑表单最大 1120px，节点/路由/出站工作区最大 1440px；达到 1200px 时章节导航约 200px。基础字段至少 1024px 可双列，复杂配置占整行。底部操作栏为 sticky，含安全区留白，最后字段保持可见。手机筛选器与刷新提示独立换行；主要操作实际命中区域至少 44×44px。

全后台列表路由为 `/`、`/machines`、`/nodes`、`/users`、`/plans`、`/access-groups`、`/subscription`、`/access-ips`、`/routes`、`/outbounds`、`/history`、`/settings`、`/audit`。七类资源使用各自 `/new` 与 `/:id/edit`，节点复制保留 `/nodes/new?copy=:id`。所有编辑均在认证边界内。

## Elevation & Depth

普通工作区和卡片通过实色背景及 1px 边框区分，不叠加装饰玻璃、模糊和光晕。仅菜单、确认和详情窗口使用 `0 16px 48px rgb(15 23 42 / 18%)`；移动导航使用 `0 16px 32px rgb(15 23 42 / 12%)`。公共登录/门户保留自己的旧层次，不套用后台规则。

控件反馈使用 140ms ease 的背景与边框过渡；章节定位在允许动态效果时平滑滚动。减少动态效果偏好关闭动画/过渡并使用直接定位。加载图标旋转属于等待反馈，不用于页面装饰。

## Shapes

基础控件、表单分区和列表面板采用 8px 圆角与细边框，状态标记为 5px，数量和标签底片为 4px。头像保留圆形。菜单和窗口不复用右侧滑入形态；手机详情窗口去掉边框和圆角，占满视口。

## Components

**操作与字段。** 主操作为蓝色实心按钮，普通操作中性，删除与重置保留确认。提交期间锁定控件和重复请求；失败保留输入，字段错误通过标签、说明及 `aria-describedby` 关联。按钮和链接焦点轮廓为主题色 2px、外偏移 3px；输入框为 2px 外偏移。协议错误摘要可聚焦。选择、插入光标、滚动条和数字都有后台主题规则。

**列表与服务器卡片。** 页头主操作、分类数量、搜索与组合筛选连接到同一数据集。卡片和表格共享筛选、记录和动作；隐藏地址覆盖可见文本及辅助描述。离线显示心跳/采样时间，缺少指标为“—”。真实分页沿用服务端口径，不能把当前页当成全部。

**完整编辑页。** `ResourceEditorPage` 复用资源定义、转换和专用字段；节点使用专用完整表单。可刷新和直达；不依赖上一页对象。安全 `returnTo`、列表 URL、会话内滚动/焦点恢复与未保存保护共同保留工作上下文。Escape 关闭菜单和确认，不关闭编辑页。系统设置保留八类分区和独立保存，切换不丢草稿。

**发布状态。** 保存草稿不宣称运行改变。发布成功为“配置已提交，等待 Agent 应用”；已应用版本由 Agent 回报。保存成功而发布失败保留记录，新节点进入其编辑地址，重试只发布。

**编辑与故障恢复。** 原管理员会话过期后在内存保留隐藏的编辑页面，重新认证后由用户手动重试；换管理员清理旧输入。配置版本冲突保留本页内容，查看最新配置和确认后重新加载是独立操作，不能自动覆盖。缺少配置版本时禁止盲写。错误显示请求 ID 并提供复制诊断，持续到用户关闭；超时没有服务端 ID 时不伪造。

**版本历史。** `/nodes/:id/history` 为完整页面，桌面左侧选择版本，右侧比较；平板和手机上下排列。差异与完整结构隐藏密码、令牌、服务端密钥和用户凭据。早期不完整记录仅可查看；恢复完整快照只写草稿，明确说明共享路由绑定将被解除，确认发布前不改变运行服务。恢复期间阻止其他页面导航。

**授权上下文。** 账号和套餐显示可跳转的权限组、绑定与已发布节点数，并解释到期、停用或额度不足；数量表示所标注的范围，不把“已发布”宣称为“已应用”。关联加载失败显示未知和重试。门户额度用尽仍显示真实用量及恢复条件。

**详情与确认。** 审计/IP 最大 1120px，安装命令最大 960px；桌面最大高 90dvh、正文滚动，低于 768px 全屏。窗口约束焦点、Escape 关闭并恢复触发点。只读窗口不嵌入资源编辑，危险提交保留关闭保护。

**公共页面。** `/edu` 继续使用既有账户中心布局、白色面板、中性黑色命令和绿色状态；深色与独立主题偏好保留。管理员代入只读，不显示密码/重置/导入操作。登录和门户的专属宽度、字号、间距以 `public-surfaces.css` 和对应组件为准，后台修改不得扩散。门户基线像素比较一致。

## Do's and Don'ts

- Do 使用真实字段、明确状态文字、稳定操作位置、完整编辑路由和可恢复的输入。
- Do 把现有协议表单、权限保护、敏感值掩码、CSRF/Cookie 与受保护分块视为兼容边界。
- Do 在 1440/1024/390px、明暗主题和空态/错误态检验任务流程。
- Don't 恢复右侧编辑抽屉、旧侧栏或后台玻璃层；不要持续堆叠历史 CSS 覆盖。
- Don't 新增探测、历史曲线、支付或后台 RBAC，也不要混淆权限组与管理员权限。
- Don't 持久化表单密码/令牌/证书、制造零值/总数，或把已提交配置标成 Agent 已应用。

源码与验证边界：`web/src/admin-console.css`、公共 `public-surfaces.css`、`AdminTheme`、`AdminNavigation`、`AppShell`、`ui` 及对应页面。实现及回退见 `design-system/admin-console-migration.md`；证据见 `design-system/admin-console-2026-11-03/`。首轮审查三项修正最终 scoped ship，不表示穷尽所有协议联机及生产安全验证。无新增产品栅格图片。
