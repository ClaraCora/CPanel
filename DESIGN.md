---
name: CPanel Glass Operations Console
description: A dense dark operations workbench built from translucent graphite, precise status light, and predictable resource workflows.
colors:
  clear-primary: "#3767d0"
  clear-primary-hover: "#315cbb"
  clear-command-text: "#ffffff"
  clear-dark-surface: "#10151f"
  clear-dark-panel: "#171e2a"
  clear-dark-panel-strong: "#1e2837"
  clear-dark-foreground: "#edf2fa"
  clear-dark-secondary: "#a1afc2"
  clear-dark-border: "#2c3749"
  clear-dark-border-strong: "#49617f"
  clear-dark-accent: "#82acff"
  clear-dark-active: "#263c60"
  clear-dark-hover: "#1c2a40"
  clear-dark-success: "#83d9b1"
  clear-dark-warning: "#f2c782"
  clear-dark-danger: "#ffa4b0"
  clear-dark-success-bg: "#203b34"
  clear-dark-warning-bg: "#3c3222"
  clear-dark-danger-bg: "#412a32"
  clear-light-surface: "#f3f6fb"
  clear-light-panel: "#ffffff"
  clear-light-panel-strong: "#edf1f7"
  clear-light-foreground: "#243047"
  clear-light-secondary: "#586b83"
  clear-light-border: "#dce3ed"
  clear-light-border-strong: "#a6b7ce"
  clear-light-accent: "#345fc5"
  clear-light-active: "#edf2ff"
  clear-light-hover: "#f2f6fd"
  clear-light-success: "#256448"
  clear-light-warning: "#845a1a"
  clear-light-danger: "#a43c4d"
  clear-light-success-bg: "#e8f4ee"
  clear-light-warning-bg: "#fff3df"
  clear-light-danger-bg: "#ffedf0"
typography:
  clear-title:
    fontSize: "26px"
    fontWeight: 650
    lineHeight: 1.3
    letterSpacing: "0"
  clear-body:
    fontFamily: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "PingFang SC", "Microsoft YaHei UI", system-ui, sans-serif'
    fontSize: "13px"
    lineHeight: 1.5
  clear-machine-data:
    fontFamily: '"Cascadia Mono", Consolas, monospace'
    fontSize: "12px"
    letterSpacing: "0"
rounded:
  clear-chip: "4px"
  clear-status: "5px"
  clear-input: "6px"
  clear-button: "7px"
  clear-menu: "8px"
  clear-list: "10px"
  clear-editor: "0"
spacing:
  clear-xs: "4px"
  clear-sm: "8px"
  clear-md: "12px"
  clear-lg: "16px"
  clear-panel-inset: "19px"
  clear-xl: "24px"
components:
  clear-button-primary:
    backgroundColor: "{colors.clear-primary}"
    textColor: "{colors.clear-command-text}"
    rounded: "{rounded.clear-button}"
    padding: "7px 12px"
  clear-button-primary-hover:
    backgroundColor: "{colors.clear-primary-hover}"
  clear-button-secondary:
    backgroundColor: "{colors.clear-dark-panel}"
    textColor: "{colors.clear-dark-foreground}"
    rounded: "{rounded.clear-button}"
    padding: "7px 12px"
  clear-search:
    backgroundColor: "{colors.clear-dark-surface}"
    rounded: "{rounded.clear-input}"
    padding: "7px 10px 7px 33px"
  clear-category-selected:
    textColor: "{colors.clear-dark-accent}"
    height: "52px"
  clear-status-success:
    backgroundColor: "{colors.clear-dark-success-bg}"
    textColor: "{colors.clear-dark-success}"
    rounded: "{rounded.clear-status}"
    padding: "3px 7px"
  clear-list:
    backgroundColor: "{colors.clear-dark-panel}"
    rounded: "{rounded.clear-list}"
  clear-editor:
    backgroundColor: "{colors.clear-dark-panel}"
    rounded: "{rounded.clear-editor}"
    width: "min(480px, 100%)"
    height: "100dvh"
  clear-menu:
    backgroundColor: "{colors.clear-dark-panel}"
    rounded: "{rounded.clear-menu}"
    padding: "6px"
---

# Design System: CPanel

## Overview

**Creative North Star: "The Illuminated Control Desk."** CPanel is an operations surface used by one administrator to monitor and configure Corade infrastructure. Its layout stays dense and familiar: grouped navigation, resource tables, full editors, drawers, forms, and explicit state. Its new identity comes from graphite glass, restrained spectral light, and crisp luminous status feedback.

The interface is dark because the administrator may keep it open beside terminals during long or low-light maintenance sessions. Cyan-blue light identifies commands, green confirms healthy runtime state, amber marks pending work, and rose marks failures. Glass is functional: it separates persistent navigation, working surfaces, and temporary overlays while preserving spatial context.

`/edu` is intentionally a separate, light account-center surface for subscribers. It keeps the same compact operational hierarchy but does not inherit the dark administration shell: white panels, quiet neutral borders, black commands, and restrained green status feedback make subscription copying and single-node import immediately legible on both desktop and phone.

No topology, relationship, force, or connection diagrams are permitted. Lists and forms remain the only management surfaces.

The user-selected **Clear A / 清晰表格** is a local exception for `/machines`, `/nodes`, `/users`, `/plans`, and `/access-groups`, and their explicitly marked local editor/menu overlays. Its solid blue-gray list treatment overrides the incumbent only within `.clear-resources`, `.clear-resource-editor`, and `.clear-resource-menu`. The frontmatter's `clear-*` tokens describe this exception. The overview, navigation shell, full node editor, other resource pages, and `/edu` retain their incumbent authority. See `.impeccable/surfaces/resource-lists.md` and `web/src/resource-clear.css`.

**Key Characteristics:**

- Dense, familiar list-and-form operations.
- Incumbent graphite glass outside the five selected resource lists.
- Solid Clear A lists with explicit dark/light palettes and real counts.
- Visible Chinese status labels, focus, and genuine operations.

## Colors

- **Canvas:** `#07090F`
- **Raised Canvas:** `#0C1018`
- **Glass Surface:** `rgba(17, 23, 34, 0.72)`
- **Strong Glass:** `rgba(22, 29, 43, 0.88)`
- **Foreground:** `#F4F7FB`
- **Secondary Text:** `#9AA6B8`
- **Muted Text:** `#718096`
- **Rule:** `rgba(190, 214, 255, 0.12)`
- **Rule Strong:** `rgba(190, 214, 255, 0.20)`
- **Primary Cyan:** `#40D9FF`
- **Primary Blue:** `#4F7CFF`
- **Violet:** `#A98BFF`
- **Success:** `#5FE0A1`
- **Warning:** `#FFC96B`
- **Danger:** `#FF6F91`

Gradients belong to page-scale light fields, primary commands, and small identity marks. Text is always solid. Passive surfaces never receive saturated color fills.

### Clear A local palette

Use `clear-primary` with white command text in both themes; its white-text contrast is about 5.23:1 (hover about 6.21:1). The theme's accent is for links, selected categories, meters, and focus. Status foregrounds use their matching `*-bg` fills; information uses accent on active fill, neutral uses secondary on strong panel. Keep the Chinese state label.

The explicit `clear-light-*` palette replaces the corresponding `clear-dark-*` roles under `html[data-theme="light"]`: surface, panel, panel-strong, foreground, secondary, border, border-strong, accent, active, hover, success, warning, danger, and each semantic background. CSS `--text` aliases foreground; `--focus` aliases accent. Primary and primary-hover remain shared. Dark component tokens in frontmatter describe the default variant; light uses this role-for-role substitution.

**The Local Override Rule.** Keep Clear A tokens on the three local scopes; never promote them to `:root` or recolor the shared shell.

## Typography

Use the native Chinese UI sans stack for all interface copy. Use the system monospace stack only for IDs, IP addresses, ports, revisions, traffic values, timestamps, hashes, and configuration snippets.

- **Page Title:** 24-26px, 650-680 weight.
- **Section Title:** 16-17px, 650 weight.
- **Body and Controls:** 12-14px, 400-650 weight.
- **Metadata:** 10-12px with sufficient contrast.
- **Machine Data:** 10-13px monospace with tabular numerals.

All letter spacing is `0`. Hierarchy comes from scale, weight, color, and spacing.

Clear A titles use `clear-title` (25px on phones). Resource names are 13px/600, compact names 12px, phone names 14px; supporting IDs are 11px on a separate wrapping line. Desktop table text and controls are 12px; metadata is 11px. Technical values use `clear-machine-data` and tabular numerals. Editor headings are 20px, labels/fields 13px, and helper/error context 12px.

## Layout

The current application composition is canonical. Desktop uses a 216px navigation rail and a flexible content column. Tablet collapses the rail to 76px. Mobile uses a dismissible drawer. Resource pages open as lists with search, filters, stable table columns, and explicit actions. Complex node and route editors remain dedicated pages; shorter edits remain drawers.

Spacing follows the existing 4px rhythm. Containers use 8-12px radii. Do not introduce decorative cards or cards inside cards.

### Clear A responsive lists

The title and add action lead one connected panel: categories with real counts, search and filters, records, then the real visible/total footer. Desktop page padding is 30px 28px 36px; category/toolbar minimum heights are 52px/70px. Headers are 39px high. Rows are nominally 76px, with 13px 11px cell padding and a 19px outer inset; long content can grow. Actions occupy the stable final column.

At **1100px and below**, page gutters reduce to 20px, headers wrap, cell side padding becomes 8px, and names, status/action columns, and meter grids tighten. At **700px and below**, page padding becomes 24px 16px and search spans the toolbar. Each record becomes two labeled columns with a 14px by 12px gap and 16px 14px padding; identity and actions span both columns. `data-label` supplies visible cell labels while the visually hidden table header remains available to assistive technology. Header commands, search, filters, row actions, and menu items use 44px minimum touch targets where implemented.

Local editors are single-column right-side drawers: width `min(480px, 100%)`, height 100dvh, 24px header/body padding (20px on phones). Dedicated full node editors and unscoped drawers retain their existing layout.

## Elevation & Depth

Persistent work surfaces use translucent graphite, backdrop blur, one low-contrast edge, and a directional dark shadow. The sidebar and mobile header use stronger glass to maintain navigation contrast. Drawers, menus, dialogs, and toasts use the strongest blur and elevation.

Glow is reserved for active navigation, primary commands, healthy status beacons, keyboard focus, and subtle pointer or hover response. It must not obscure text or table rules.

Clear A lists and controls are flat solid surfaces with one-pixel borders and no resting/hover shadow. Only local overlays retain shadows: drawer `-12px 0 40px #0003`, menu `0 12px 24px #0003`.

### Incumbent motion

One slow ambient light sweep animates behind the application. Page headers enter with a short blur-to-focus transition; healthy status dots breathe; loading skeletons use a bounded shimmer. Hover and press transitions use exponential ease-out. All animation stops under `prefers-reduced-motion`.

Clear A disables page/drawer entrance and status-dot animation. Motion is limited to hover/focus feedback; buttons transition color/background for 180ms with `ease`, without lift, glow, or sweep. Reduced-motion disables descendant animations and transitions in all three local scopes. Preserve operational loading behavior without adding decorative motion.

## Shapes

The incumbent container radii remain unchanged. Clear A uses a 10px outer list, 7px buttons, 6px fields, 5px status/icon controls, 4px count/role/node chips, and an 8px menu. Records are square divided strips. The local drawer has square corners and one left border.

## Components

### Incumbent buttons

- Primary commands use a cyan-blue-violet gradient with dark text and a controlled light sweep on hover.
- Secondary commands use frosted graphite with one neutral edge.
- Ghost actions become visible on hover without gaining a floating card silhouette.
- Destructive commands use rose only inside explicit destructive actions.
- Icon buttons remain square, have accessible names, and gain a restrained glass highlight.

### Incumbent tables

- Toolbars and tables visually connect as one work surface.
- Headers use a darker translucent band and remain compact.
- Rows use a single divider and a low-energy cyan-violet hover wash.
- IDs and measurements remain monospace; resource names remain sans-serif.

### Incumbent forms

- Fields use dark inset surfaces, visible labels, cyan focus rings, and explicit validation.
- Configuration groups are separated by rules, not nested cards.
- Segmented controls use one selected luminous segment.
- Sticky editor actions use strong glass so content remains visible beneath them.

### Incumbent status

Every semantic color appears with a Chinese text label. Healthy status dots may breathe slowly. Error and pending states never depend on color alone.

### Clear A controls and records

Primary commands use the solid primary/white pair; secondary commands use panel, foreground, and border, with strong-panel/strong-border hover. Buttons are at least 36px high, with 7px 12px padding; primary weight is 600. Persistent edit text uses accent and underlines on hover. Named overflow triggers are 30px by 32px, increasing to 44px square on phones.

Category buttons form a labeled group with `aria-pressed`; active text and its two-pixel underline use accent, count chips use strong panel. Combine category/search/detail filters and retain a clear-filter recovery. Status badges use a static 5px dot, Chinese label, and 3px 7px padding. Protocol labels are unfilled accent monospace. Meter tracks are 3px high with visible numeric values.

Search uses canvas inset, border, and a 6px radius. Preserve **33px inline-start padding**: `.app-shell .page.clear-resources .search-box input` intentionally wins legacy input specificity with `padding-inline: 33px 10px`. Its minimum height is 36px (44px on phones). Editor fields are at least 40px with 9px 11px padding. Focus uses accent border and a 2px outline, offset 3px in lists and 2px in local overlays; visible labels, helpers, and validation remain.

The connected list reports actual visible/total records without invented pagination. Empty, loading, refresh/error, and filtered-empty states remain real. Explicitly scoped drawers and menus retain original focus trapping/restoration, Escape/menu keyboard handling, scroll locks, unsaved-change protection, destructive confirmations, administrator protection, installation/upgrade/reset operations, and delegated read-only subscription access. Shared confirmation dialogs retain incumbent treatment. Menu items are at least 36px high (44px on phones).

### Portal quality bar

The `/edu` portal is judged against a compact account-center reference, not the dark control desk. Its native devices are a 58px utility header, four compact summary cells, a subscription row with one copy action, one-click node rows, and a right-side traffic/security stack. Desktop preserves a wide left working column and narrow status column; phone restacks the same order without horizontal overflow. Form fields remain white inset controls with visible focus rings; password autofill must never switch to a dark surface. Delegated administrator mode is visibly read-only and exposes only copy actions.

## Do's and Don'ts

- Preserve all existing content, roles, workflows, and responsive composition.
- Never add topology graphs, relationship maps, force diagrams, or decorative connection lines.
- Never copy Xboard styling or navigation structure.
- Never nest cards or turn every form section into an isolated floating card.
- Never use gradient text.
- Never let glow replace legibility, labels, or focus indication.
- Do keep Clear A scoped to the five selected lists and their marked local overlays, including the explicit light palette.
- Do preserve the overview, shared navigation, full node editor, other resource pages, and `/edu`.
- Do preserve 33px search text inset, labeled mobile records, real counts, and genuine accessible operations.
- Don't introduce shipping raster assets or invented pagination for these lists.
