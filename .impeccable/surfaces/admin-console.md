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

Implemented in `e0d2ef7`, tested in `3741c05` and corrected in `cbf5224`. Root `DESIGN.md` and `.impeccable/design.json` now describe the replacement system. Old resource/operations briefs explicitly defer here for administration and preserve only valid public portal guidance.

Primary source is `web/src/admin-console.css`; public style consolidation is `public-surfaces.css`. Seven old overlay stylesheets and all administration Drawer references were removed. `ResourceEditorPage` and `NodeEditorPage` implement complete editing routes. Current mobile filters have independent rows, actual primary text controls are at least 44×44px, and node client/server errors restore focus to described targets or a focusable summary.

Evidence: `design-system/admin-console-migration.md` and `design-system/admin-console-2026-11-03/`. Frontend typecheck/build and 39 tests passed; Go HTTP/web regressions passed with isolated PostgreSQL Agent enrollment coverage. Final reviewer disposition is scoped ship for F1–F3 resolved, not an exhaustive live-protocol/production security audit. No new shipping raster assets.
