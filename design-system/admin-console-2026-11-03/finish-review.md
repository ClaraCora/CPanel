# CPanel administrator console finish review

## 1. Capture validity

The evidence is valid for a finish review. `captures-final.json` contains all 120 required route/width/theme combinations: 13 administrator pages and seven editors at 1440, 1024 and 390 pixels in light and dark mode. All referenced files exist; the manifest reports no document horizontal overflow and no browser page errors. The twelve labeled round2 contact sheets were inspected across the complete matrix. Full-size representative captures were also inspected for desktop server cards, tablet node tables and forms, mobile account records, mobile route editing, overview, dark settings, all six required audit/IP detail captures, error/empty states, and both node-editor bottom states.

The regular captures use the requested 1440×1000, 1024×1000 and 390×844 browser viewports. Long pages are full-document images, so their PNG height can exceed the viewport height. The sticky footer appearing partway through a full-document editor image is a capture artifact, not evidence of an obstructed last field. The separate bottom-state images show the last node option and reachable actions without overlap. The detail recaptures are actual viewport images: mobile dialogs occupy 0,0,390,844; tablet and desktop dialogs are centered and readable.

I independently compared the preserved public screenshots. The portal has no pixel differences from its baseline. The login difference is confined to bounds (576,488)–(730,573), consistent with populated inputs. No public appearance redesign is indicated.

## 2. Direction/requirements assessment

The implementation follows the approved plan and `.impeccable/surfaces/admin-console.md`: 64px horizontal navigation, the prescribed information order, light default, blue actions, graphite dark theme, thin borders, 8px corners, Chinese system typography, centered workspace, 3/2/1 server cards and a table alternative. Mobile tables become labeled records. All seven create/edit resource flows have authenticated full-page routes; node copy retains its own route. Complex editors provide section navigation and bottom actions. Read-only audit/IP details use centered dialogs and full-screen mobile presentation.

Source inspection supports the intended workflow: resource-specific safe `returnTo`, URL-backed list filters, in-memory position restoration, dirty guards for application navigation, browser history and unload, isolated editor identity, cancellation of obsolete loads, explicit missing/error states, and preservation of settings drafts. Node draft saving and publishing remain separate; publication copy waits for Agent application, and partial save/publish failure keeps the saved node and retries publication. Address masking covers rendered table/card values. Installation credentials are generated only by the explicit action. The reviewed screens do not invent probes, metric history, billing or administrator RBAC.

`workflows.json`, `real-workflows.json`, `portal-session.json` and `session-expiry.json` provide useful behavior evidence, including real backend CRUD/refresh/readback for all seven resources, missing IDs, logout/protected routes, delegated read-only portal access and mounted-session expiry. The reported passing typecheck, 36 tests, build and Go/PostgreSQL regressions support this review at their recorded scope; they do not establish every protocol or network-failure branch.

## 3. Material findings with exact affected paths

### F1 — P2: Keep the mobile server region selector readable

At 390px, the region selector collapses to a roughly 25px chevron-only control. Its current value and purpose disappear beside the freshness text and view controls. This is visible in `round2/machines-390-light.png`, `machines-390-dark.png`, `empty-machines-390-light.png` and `no-results-390-light.png`. The filter still exists, but an administrator cannot identify the selected region from the closed control.

Affected implementation: `C:/Users/ccios/Desktop/CPanel/device-platform/web/src/admin-console.css:478` and `:479`, together with the shared toolbar at `C:/Users/ccios/Desktop/CPanel/device-platform/web/src/pages/ResourceListPage.tsx:225`. The mobile rule gives the filter `flex: 1` and removes the select minimum width while freshness and view controls consume the same row. Give the region filter sufficient independent width or its own mobile row. Verify its label/value with both ordinary and empty data, and with stale freshness copy.

### F2 — P2: Give primary mobile text actions a 44px target

The shared `clear-row-edit` style has zero padding and no minimum target size. Mobile “编辑”, audit “详情” and “查看 IP” therefore retain small text-sized hit areas, even though adjacent icon controls grow to 44px. These are the principal actions for entering the full editors and details, and the approved plan explicitly requires at least 44px primary mobile touch targets.

Affected implementation: `C:/Users/ccios/Desktop/CPanel/device-platform/web/src/admin-console.css:167`; uses include `C:/Users/ccios/Desktop/CPanel/device-platform/web/src/pages/ResourceListPage.tsx:216`, `C:/Users/ccios/Desktop/CPanel/device-platform/web/src/pages/NodesPage.tsx:162`, `C:/Users/ccios/Desktop/CPanel/device-platform/web/src/pages/AuditPage.tsx:30` and `C:/Users/ccios/Desktop/CPanel/device-platform/web/src/pages/UserAccessIPsPage.tsx:87`. Add a scoped mobile minimum width/height and suitable inline alignment without changing the pinned visual treatment. Check actual link/button bounds, not the enclosing action row.

### F3 — P2: Make node validation move to the first recoverable error

`NodeEditorPage.validate()` focuses an element whose ID equals the first error key. This works for the few basic controls with explicit IDs, but `config` and `endpoint_0_name`, `endpoint_0_host`, etc. have no corresponding control IDs. Endpoint errors also lack an error-to-control ARIA association. API field errors only call `setErrors`, with no focus recovery. When saving from the bottom of this long editor, an invalid endpoint or protocol configuration can leave focus on the save action while the relevant error remains elsewhere on the page. This misses the plan’s first-error location and correctly associated errors requirements.

Affected implementation: `C:/Users/ccios/Desktop/CPanel/device-platform/web/src/pages/NodeEditorPage.tsx:149`, `:165`, `:212` and the endpoint fields at `:269`; protocol error presentation is at `C:/Users/ccios/Desktop/CPanel/device-platform/web/src/components/NodeConfigForm.tsx:335`. Map endpoint errors to stable IDs and their descriptions; provide a focusable protocol error/group target when a precise field is unavailable; use the same first-error recovery after client and server validation. Verify an invalid endpoint and an invalid protocol configuration while scrolled to the bottom, preserving all input.

## 4. Craft-floor assessment

The pinned card/metric structure, system font and 8px radius take precedence over generic craft-floor preferences and should remain. The visual system is coherent across both themes: readable token choices, clear type hierarchy, thin separators, restrained blue emphasis, consistent Lucide icons and monospace technical values. Empty, missing and retry states name the situation and provide the next action. Detail dialogs have bounded scrolling, focus trapping, Escape handling and focus restoration. Focus rings, selection, caret, scrollbar colors and tabular values are explicitly styled; reduced-motion behavior is present. No shipping raster assets require provenance.

The detector result is `[]` and was not rerun. I found no reason to reopen the approved design direction or require a comp/QUALITY BAR artifact. The remaining craft-floor failures are the specific responsive control and error-recovery issues above, not a need for new visual concepts.

## 5. Disposition and scope

Disposition: **fix**.

The architecture and visual direction are suitable to ship after F1–F3 are corrected. Make one scoped fix batch, verify the mobile region toolbar and actual text-action bounds, exercise the two node validation cases plus server field-error recovery, and recapture the affected 390px light/dark states. Recheck the node editor bottom state if its error markup changes layout. Preserve the public baseline and existing API/security/Agent contracts. No broader redesign or unrelated capability expansion is requested by this review.

This review used the approved plan, direction contract, craft floor, source and saved captures/evidence only. It did not browse the running application, modify source, read shipped agent-definition files or make production mutations.
