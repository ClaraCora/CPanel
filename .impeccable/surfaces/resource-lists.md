# Resource lists — selected Clear A

Mode: Operate. Approved on 2026-11-01 by the user's “选择清晰A”.

Scope: `/machines`, `/nodes`, `/users`, `/plans`, `/access-groups`. This original five-page approval preserves the overview and navigation shell. The subsequent approved nine-surface extension is recorded in `remaining-operations.md`; its operation and portal changes do not replace this brief's five-page rules or 480px resource drawers. This is a code-led implementation of the user's selected HTML prototype, not a new direction round.

## Direction contract

THESIS: A single readable table surface joins classification, search, filters, and real records. Names and supporting data sit on separate lines; actions occupy a stable final column.

OWN-WORLD: Blue-gray canvas `#10151f`, solid panel `#171e2a`, rules `#2c3749`, foreground `#edf2fa`, secondary `#a1afc2`, links `#82acff`, primary `#3767d0`. Preserve the existing light theme with scoped equivalents.

STORY: An administrator scans operational state, narrows real records, edits existing forms, and deliberately publishes or changes resources. Plans describe manually allocated resources; access groups define node membership. Admin protection and delegated read-only subscription access persist.

FIRST VIEWPORT: Page title and add action above one panel; classification with real counts, search and secondary filters, labeled columns, nominal 76px rows, and actual visible/total counts. On phones each row becomes two labeled columns, with name and actions spanning the width.

FORM: User-selected A “清晰表格”, from the three code prototypes in `../design-proposals-2026-11-01/resources/`; no concept reroll. Signature interaction: combined classification/search/detail filtering with a single clear-filter recovery. Motion is limited to hover/focus feedback; respect reduced motion.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Implementation truth

`web/src/resource-clear.css` is scoped to `.clear-resources`, `.clear-resource-editor`, and `.clear-resource-menu`. The shared Drawer and RowMenu accept optional classes; their default behavior remains unchanged. Original APIs, confirmations, unsaved-change protection, focus trap, installation/upgrade/reset operations, and subscription delegation remain in place. Lists show real record counts without invented pagination. No new shipping raster assets.
