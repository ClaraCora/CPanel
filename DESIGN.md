---
name: CPanel Operations Workbench
description: A quiet, dense administration interface built around predictable lists, filters, detail views, and configuration forms.
---

# Overview

**Creative North Star: "The Maintainer's Workbench."** CPanel is a practical operations surface for one administrator. Its identity comes from precise information structure, compact controls, and clear system states, not diagrams or decorative effects.

The interface uses a light navigation rail and a soft gray working area, following Apple's restrained system color language. Resource lists are the default surface. Creation and editing use full-height drawers for short resources and dedicated pages for complex node, route, and system configuration. Motion is limited to drawers, menus, loading feedback, and status changes.

**Key Characteristics:**

- Dense tables with stable columns, explicit filters, pagination, and batch selection.
- Light work surfaces with Apple blue actions and independent semantic state colors.
- Forms grouped by operational meaning, with visible labels and inline validation.
- Consistent list, detail, create, edit, archive, publish, and rollback patterns.
- No topology graphs, relationship canvases, decorative dashboards, or marketing layouts.

# Colors

The palette is restrained and light because administrators may keep the interface open for long work sessions. A quiet off-white navigation rail separates global wayfinding from the editable work surface without creating a heavy dark frame.

- **Navigation:** `#FBFBFD` for the permanent sidebar and top-level chrome.
- **Navigation Active:** `#E8F2FD` with blue text for the current destination.
- **Primary:** `#0071E3` for primary commands, active tabs, selected rows, and keyboard focus.
- **Primary Hover:** `#0077ED` for primary command hover and pressed states.
- **Work Surface:** `#F5F5F7` for the application background.
- **Panel:** `#FFFFFF` for tables, fields, drawers, and editable surfaces.
- **Foreground:** `#1D1D1F` for headings and primary content.
- **Secondary Text:** `#6E6E73` for descriptions and metadata.
- **Border:** `#D2D2D7` for table rules, form groups, and control outlines.
- **Success:** `#248A3D` for online, enabled, and synchronized states.
- **Warning:** `#B25000` for pending, degraded, and expiring states.
- **Danger:** `#D70015` for offline, failed, destructive actions, and errors.
- **Information:** `#0071E3` for neutral progress and system notices.

**The Command Color Rule.** Apple blue identifies interaction and focus; it does not color passive containers.

**The Explicit State Rule.** Every semantic color appears with a Chinese label and, where space permits, a Lucide icon.

# Typography

Use the native Chinese UI sans stack for headings, navigation, forms, and tables. Use a system monospace only for IDs, IP addresses, ports, revisions, timestamps, hashes, and configuration snippets.

- **Page Title:** 24px, 650 weight, compact and left aligned.
- **Section Title:** 16px, 650 weight.
- **Body and Controls:** 14px, 400-600 weight, 1.5 line height.
- **Table Metadata:** 12px minimum, with sufficient contrast.
- **Data:** 12-13px monospace for machine-readable values only.

All letter spacing is `0`. Hierarchy comes from size, weight, spacing, and alignment.

**The Data Only Rule.** Monospace is never used to make ordinary interface text look technical.

# Layout

Desktop uses a 216px navigation rail and a flexible content column. Page headers, filter bars, tables, pagination, and bulk actions align to a common horizontal grid. The main surface stretches to use available width and does not wrap every section in a card.

Tables maintain stable column widths, sticky headers when rows scroll, and a dedicated rightmost action column. Detail pages use tabs below the resource header. Complex editors use a readable two-column form at large widths and one column below tablet width. On phones, tables become labeled record rows and only monitoring plus emergency actions are guaranteed.

Spacing uses a 4px base rhythm: 4, 8, 12, 16, 24, and 32px. Standard table rows are 48px; compact controls are at least 36px on desktop and 44px on touch layouts.

**The List First Rule.** Every managed resource opens to a list with search, filters, visible status, and a clear create command.

**The Form Depth Rule.** Short edits use a drawer; node, routing, and system configuration use dedicated pages because they require validation and review.

# Elevation & Depth

Persistent content is flat and separated by borders or tonal bands. Shadows are reserved for drawers, menus, dialogs, and drag states. Cards are used only for genuinely repeated overview summaries or contained alerts, never as page-section wrappers.

**The One Separator Rule.** A persistent surface uses either a border or a shadow, never both.

# Shapes

Controls and containers use a 6px radius. Status chips may use a small capsule shape. Icon buttons are square. Tables, page bands, and form sections are not rounded floating panels.

# Components

### Buttons

- Primary buttons use Apple blue fill, white text, 36px desktop height, and a leading Lucide icon when the command has one.
- Secondary buttons use a white surface and a single neutral border.
- Destructive buttons use danger color only inside an explicit danger zone or confirmation.
- Icon-only buttons have tooltips and accessible names.

### Tables

- Headers use a muted work-surface fill and remain sticky inside scroll regions.
- Rows use 48px height, a single bottom divider, and a quiet hover fill.
- Selection, status, and row actions have stable dedicated columns.
- Empty, loading, error, filtered-empty, and pagination states are built in.

### Inputs / Fields

- Every field has a persistent visible label; placeholders provide examples only.
- Inputs use white background, neutral border, 6px radius, and a blue focus ring.
- Validation appears beside the field and names both the problem and recovery.
- Protocol-specific fields appear progressively after protocol and kernel selection.

### Navigation

- The left rail groups resources under Overview, Infrastructure, Access, Traffic, and System.
- Active navigation uses a pale blue fill plus a narrow blue marker.
- Tablet navigation collapses to an icon rail; mobile navigation opens as a dismissible drawer.

# Do's and Don'ts

### Do:

- **Do** make add, edit, enable, disable, archive, publish, and rollback predictable across resources.
- **Do** keep node protocol, server, port, status, revision, and last report visible in the node list.
- **Do** show publish validation and affected resource counts before applying configuration.
- **Do** preserve filters and pagination when returning from an editor.
- **Do** support keyboard navigation, visible focus, and reduced motion.

### Don't:

- **Don't** use topology graphs, relationship maps, force diagrams, or decorative connection lines.
- **Don't** copy Xboard's API, page structure, names, or styling; only the familiar add/edit resource workflow is retained.
- **Don't** hide high-frequency row actions behind hover-only affordances.
- **Don't** nest cards or turn every form section into a floating card.
- **Don't** use gradients, glass, glow, animated backgrounds, or decorative React Bits effects.
