---
name: CPanel Glass Operations Console
description: A dense dark operations workbench built from translucent graphite, precise status light, and predictable resource workflows.
---

# Overview

**Creative North Star: "The Illuminated Control Desk."** CPanel is an operations surface used by one administrator to monitor and configure Corade infrastructure. Its layout stays dense and familiar: grouped navigation, resource tables, full editors, drawers, forms, and explicit state. Its new identity comes from graphite glass, restrained spectral light, and crisp luminous status feedback.

The interface is dark because the administrator may keep it open beside terminals during long or low-light maintenance sessions. Cyan-blue light identifies commands, green confirms healthy runtime state, amber marks pending work, and rose marks failures. Glass is functional: it separates persistent navigation, working surfaces, and temporary overlays while preserving spatial context.

`/edu` is intentionally a separate, light account-center surface for subscribers. It keeps the same compact operational hierarchy but does not inherit the dark administration shell: white panels, quiet neutral borders, black commands, and restrained green status feedback make subscription copying and single-node import immediately legible on both desktop and phone.

No topology, relationship, force, or connection diagrams are permitted. Lists and forms remain the only management surfaces.

# Colors

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

# Typography

Use the native Chinese UI sans stack for all interface copy. Use the system monospace stack only for IDs, IP addresses, ports, revisions, traffic values, timestamps, hashes, and configuration snippets.

- **Page Title:** 24-26px, 650-680 weight.
- **Section Title:** 16-17px, 650 weight.
- **Body and Controls:** 12-14px, 400-650 weight.
- **Metadata:** 10-12px with sufficient contrast.
- **Machine Data:** 10-13px monospace with tabular numerals.

All letter spacing is `0`. Hierarchy comes from scale, weight, color, and spacing.

# Layout

The current application composition is canonical. Desktop uses a 216px navigation rail and a flexible content column. Tablet collapses the rail to 76px. Mobile uses a dismissible drawer. Resource pages open as lists with search, filters, stable table columns, and explicit actions. Complex node and route editors remain dedicated pages; shorter edits remain drawers.

Spacing follows the existing 4px rhythm. Containers use 8-12px radii. Do not introduce decorative cards or cards inside cards.

# Material And Depth

Persistent work surfaces use translucent graphite, backdrop blur, one low-contrast edge, and a directional dark shadow. The sidebar and mobile header use stronger glass to maintain navigation contrast. Drawers, menus, dialogs, and toasts use the strongest blur and elevation.

Glow is reserved for active navigation, primary commands, healthy status beacons, keyboard focus, and subtle pointer or hover response. It must not obscure text or table rules.

# Components

## Buttons

- Primary commands use a cyan-blue-violet gradient with dark text and a controlled light sweep on hover.
- Secondary commands use frosted graphite with one neutral edge.
- Ghost actions become visible on hover without gaining a floating card silhouette.
- Destructive commands use rose only inside explicit destructive actions.
- Icon buttons remain square, have accessible names, and gain a restrained glass highlight.

## Tables

- Toolbars and tables visually connect as one work surface.
- Headers use a darker translucent band and remain compact.
- Rows use a single divider and a low-energy cyan-violet hover wash.
- IDs and measurements remain monospace; resource names remain sans-serif.

## Forms

- Fields use dark inset surfaces, visible labels, cyan focus rings, and explicit validation.
- Configuration groups are separated by rules, not nested cards.
- Segmented controls use one selected luminous segment.
- Sticky editor actions use strong glass so content remains visible beneath them.

## Status

Every semantic color appears with a Chinese text label. Healthy status dots may breathe slowly. Error and pending states never depend on color alone.

# Motion

One slow ambient light sweep animates behind the application. Page headers enter with a short blur-to-focus transition; healthy status dots breathe; loading skeletons use a bounded shimmer. Hover and press transitions use exponential ease-out. All animation stops under `prefers-reduced-motion`.

# Portal Quality Bar

The `/edu` portal is judged against a compact account-center reference, not the dark control desk. Its native devices are a 58px utility header, four compact summary cells, a subscription row with one copy action, one-click node rows, and a right-side traffic/security stack. Desktop preserves a wide left working column and narrow status column; phone restacks the same order without horizontal overflow. Form fields remain white inset controls with visible focus rings; password autofill must never switch to a dark surface. Delegated administrator mode is visibly read-only and exposes only copy actions.

# Boundaries

- Preserve all existing content, roles, workflows, and responsive composition.
- Never add topology graphs, relationship maps, force diagrams, or decorative connection lines.
- Never copy Xboard styling or navigation structure.
- Never nest cards or turn every form section into an isolated floating card.
- Never use gradient text.
- Never let glow replace legibility, labels, or focus indication.
