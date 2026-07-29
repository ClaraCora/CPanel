# CPanel Design System

**Mode:** Operate
**Density:** 9/10
**Motion:** 1/10
**Stack:** React

## Structure

- Permanent 216px desktop navigation rail.
- Full-width list and form surfaces; no page-section cards.
- Resource pages start with page header, filters, table, and pagination.
- Short resources edit in drawers; nodes, routing, and settings edit on dedicated pages.
- Desktop table rows are 48px; touch targets become at least 44px on tablet and mobile.

## Tokens

| Role | Value |
| --- | --- |
| Navigation | `#171A1F` |
| Navigation active | `#232A31` |
| Primary | `#0B6B64` |
| Primary hover | `#085650` |
| Work surface | `#F4F6F8` |
| Panel | `#FFFFFF` |
| Foreground | `#171A1F` |
| Secondary text | `#626B76` |
| Border | `#D8DDE3` |
| Success | `#1F7A55` |
| Warning | `#A86608` |
| Danger | `#C23838` |
| Information | `#3267A8` |

Spacing uses `4, 8, 12, 16, 24, 32px`. Radius is `4px`; status chips may be pills. Shadows are reserved for overlays.

## Typography

- Native Chinese UI sans for all interface text.
- 24px/650 page title, 16px/650 section title, 14px controls and body, 12px minimum metadata.
- System monospace only for IDs, addresses, ports, revisions, timestamps, and configuration.
- Letter spacing is always `0`.

## Interaction

- Use Lucide icons; icon-only buttons require accessible labels and tooltips.
- Keep visible labels on fields and validation beside the failing field.
- Use 120-180ms transitions for drawers, menus, hover, and focus only.
- Respect `prefers-reduced-motion`.
- Status never depends on color alone.

## Forbidden

- No topology or relationship diagrams.
- No gradients, glass, glow, animated backgrounds, decorative React Bits components, or hero layouts.
- No nested cards, floating page sections, or hover-only essential actions.
- No Xboard API or visual imitation.

## Verification

- Check 375, 768, 1024, and 1440px widths without horizontal page scrolling.
- Check keyboard navigation, focus visibility, loading, empty, filtered-empty, error, and disabled states.
- Maintain 4.5:1 text contrast and reserve stable dimensions for tables, toolbars, and counters.
