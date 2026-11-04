# Administrator console — 2026-11-03

Mode: Operate. Authority: the user's approved `CPanel界面重构实施方案.MD` and standalone preview at `../design-proposals-2026-11-03/`. This brief supersedes the administration portions of `resource-lists.md` and `remaining-operations.md`. Login and `/edu` retain their incumbent appearance and behavior.

## Direction contract

THESIS: A readable administration workspace organized around top navigation, operational server cards and full resource editing pages. Every configuration task gets a refreshable route; no editing drawers remain.

OWN-WORLD: Light canvas #F5F7FA, white panels, #1F2937 text and #2563EB actions; graphite #10151F and #182130 panels in dark mode. Thin borders, 8px corners, system Chinese type and monospace technical values.

STORY: Administrators scan real state, combine filters, open a complete editor, save deliberately and return to the same list context. Submitted configuration and Agent-applied configuration remain distinct.

FIRST VIEWPORT: A 64px horizontal header, centered workspace up to 1440px and title/action row. Servers default to three cards across on desktop; other resources use labeled tables. Editors show context, section navigation, a wide form and reachable bottom actions. Below 1024px navigation collapses; mobile records and fields stack.

FORM: User-pinned direction, code-led; the approved plan and existing independent preview settle the design. Signature interaction: URL-backed list state survives complete-page editing with dirty protection, focus and scroll restoration. Motion is limited to control feedback and reduced-motion-aware section scrolling.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Boundaries

13 administration routes and seven resource editors. Preserve existing API mappings, types, security and Agent contracts. No new connectivity probes, metric history, payment or RBAC capability. All illustrative data stays in existing demo mode. No shipping raster assets.


## Finished implementation

服务器与节点列表使用局部紧凑密度：分类控件最小高度 44px，筛选区上下内边距 12px，桌面表格单元上下内边距 12px；服务器卡片内边距桌面 16px、手机 12px，名称字号 15px，正文行高 1.45。CPU、内存、磁盘横向等分，每项上排标签与百分比、下排进度条，字号 12px、行高 1.3、组间距和分组上下外边距均为 12px。只显示一行最后心跳，指标采样时间保留在指标组提示中；Agent 与操作区上内边距 8px，时间区上下外边距 8px。横向指标限定在卡片内，不影响表格。手机按钮保留 44px 最小触控高度。其他页面、编辑页和公共入口不继承此局部密度。

2026-11-04 refinement: ordinary list headers use 24px top spacing and a 20px gap before the work area; ordinary table cells use 14px vertical padding. Server cards group identity, metrics, Agent and timestamps, with bottom actions aligned. Mobile node records combine report time and actions in a shared footer. Tablet overview metrics remain five columns; mobile rankings hide the secondary rank column and right-align intact quantities. Inputs retain their error styling on hover/focus; busy actions expose status and a visible spinner. Full editing routes and public-page styles remain intact. Report and demo evidence: `design-system/ui-polish-2026-11-04.md` and its sibling directory.

Implemented in `e0d2ef7`, tested in `3741c05` and corrected in `cbf5224`. Root `DESIGN.md` and `.impeccable/design.json` now describe the replacement system. Old resource/operations briefs explicitly defer here for administration and preserve only valid public portal guidance.

Primary source is `web/src/admin-console.css`; public style consolidation is `public-surfaces.css`. Seven old overlay stylesheets and all administration Drawer references were removed. `ResourceEditorPage` and `NodeEditorPage` implement complete editing routes. Current mobile filters have independent rows, actual primary text controls are at least 44×44px, and node client/server errors restore focus to described targets or a focusable summary.

Evidence: `design-system/admin-console-migration.md` and `design-system/admin-console-2026-11-03/`. Frontend typecheck/build and 39 tests passed; Go HTTP/web regressions passed with isolated PostgreSQL Agent enrollment coverage. Final reviewer disposition is scoped ship for F1–F3 resolved, not an exhaustive live-protocol/production security audit. No new shipping raster assets.
