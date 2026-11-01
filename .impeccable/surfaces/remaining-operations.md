# Remaining Clear A operations

Mode: Operate. The administrator configures subscription output, reviews access and audit records, edits routes/outbounds and full node protocols, and changes platform settings. Subscribers copy links and inspect their allowance in the independent account center.

## Direction contract

THESIS: Extend the user-approved Clear A prototypes at `../design-proposals-2026-11-01/remaining-a/` to the remaining nine live surfaces. Keep records and forms directly readable, with real API behavior.

OWN-WORLD: Inherit the existing local Clear A solid blue-gray palette, connected work surfaces, two-line identities and stable final actions. The portal uses white panels, a light neutral canvas, black commands and green status; its appearance preference remains independent.

STORY: Select a record, inspect its current values, edit safely and save explicitly. No invented connectivity, historical metric curves, payments or RBAC controls.

FIRST VIEWPORT: The page heading leads the work area. Lists combine category/search/records; settings use a compact left directory and two-column form. The node editor has a section directory beside its complete form. The portal places subscription/nodes left and traffic/account right. Under 700px, records become two labeled columns and forms stack.

FORM: Approved system extension, code-led implementation of the selected working HTML prototypes. Scope `/subscription`, `/access-ips`, `/routes`, `/outbounds`, `/history`, `/settings`, `/audit`, node create/edit/copy and `/edu`. Overview and shared shell preserve their existing authority.

SIGNATURE: Subscription settings collapse without losing the form draft; each route/outbound editor opens the selected live record. All node protocol branches and validation remain available. Delegated portal sessions allow copying only. Motion is limited to control feedback and respects reduced motion.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Implementation truth

Approved implementation on 2026-11-01 extends the existing Clear A system; `DESIGN.md` is merged rather than replaced. The initial five resource pages retain their palette, list behavior, 480px editors, and `resource-lists.md` authority. The overview and shared navigation shell retain their incumbent design.

- `/subscription`, `/access-ips`, `/history`, `/settings`, `/audit`, and full node create/edit/copy combine `.clear-resources` with `.clear-operations`; the node form additionally uses `.clear-node-editor`. `/routes` and `/outbounds` use the existing generic resource-list component with live specialized forms. `web/src/resource-clear.css` supplies shared palette/list rules; `web/src/operations-clear.css` supplies the local form, directory, detail, and full-node treatment. No administration palette is promoted to `:root`.
- The complex route/outbound and IP/audit drawers carry `.clear-resource-editor.clear-operation-editor`. The descendant `.drawer` is `min(720px, 100%)` wide; the outer fixed modal layer/backdrop still fills the viewport. The older resource drawer remains `min(480px, 100%)`. The IP detail table keeps its explicit inner horizontal scroll for additional columns.
- Settings retain all eight categories. Mounted hidden sections retain input drafts; subscription folding likewise keeps its form mounted. Subscription starts expanded above 700px and collapsed at or below 700px, with `aria-expanded`/`aria-controls`. Full node chapters and every `NodeConfigForm` branch, port/protocol checks, draft/publication paths, loading/error/dirty states and confirmations remain in place. No backend or protocol redesign is introduced.
- Subscription access, IP, history, and audit tables keep real API paging, queries, counts and error states. History distinguishes daily traffic, latest server metrics, latest node metrics, and device records. Access-IP details retain the per-account ten-address limit and explicit location lookup. Route fallback and specialized outbound settings remain existing application behavior. Audit technical values wrap; sensitive change keys remain redacted in the detail renderer.
- `/edu` uses `.edu-app.edu-clear` in `web/src/PortalDashboardPage.tsx` and `web/src/portal-clear.css`, after incumbent portal/theme styles. Light mode uses its own canvas, borders and neutral commands; dark mode maps its panel roles to Clear A dark values while retaining portal command and danger roles. `web/src/theme.ts` defaults `/edu` and `/edu/` to light and persists `cpanel.portal.theme` independently of administration's `cpanel.theme`.
- Portal subscription/node links, usage, account values, copy/import, password change and reset continue to use the real portal API and existing client behavior. Delegated sessions visibly permit copying only and hide import/reset/password controls. The reset dialog retains focus/Escape/scroll-lock behavior. UI hiding is not a claim of newly verified server authorization.

The final source uses the established native Chinese UI stack, 26px administration titles and zero tracking; the prototype's 28px administration headings, compact navigation width and other unshipped values are not normative. The portal's own 28px title, 1136px workspace and 64px utility header are local account-center values. `DESIGN.md` frontmatter owns token primitives; `.impeccable/design.json` extends those tokens with metadata and self-contained examples. No shipping raster assets were introduced.

## Evidence and limits

The implementation packet is `.impeccable/review/remaining-implementation/IMPLEMENTATION-CHECKS.md`. Its 27 recorded viewport checks cover nine pages at 1440, 1017 and 390px and report page scroll width equal to client width. The packet records 21 passing frontend tests, successful typecheck/build, selected behavior checks, and a single detector result of `[]`; these are existing implementation evidence, not checks rerun during documentation.

`finish-review.md` has disposition **ship** for the scored F1 overlay correction only: the replacement captures show the wider child drawer aligned to the viewport right edge with a full backdrop. It is not a new whole-surface, live-network or security audit. This documentation pass read the final source and evidence without browser or remote commands; it does not independently verify real mutations, server permissions, every protocol/failed-state branch, GitHub publication or SG1 deployment. Those release steps remain outside this documentation boundary.
