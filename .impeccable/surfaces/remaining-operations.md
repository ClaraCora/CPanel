# Remaining operations and preserved account center

Mode: Operate. For administration, the 2026-11-01 extension is superseded by [administrator console](admin-console.md), root `DESIGN.md` and the 2026-11-03 implementation plan. Routes `/subscription`, `/access-ips`, `/routes`, `/outbounds`, `/history`, `/settings`, `/audit` and full node editing inherit the current top-navigation/light-default system. Old 480px/720px drawers, retained sidebar/glass shell and deleted operations CSS are not current authority.

## Preserved portal authority

The `/edu` account center was excluded from the 2026-11-03 redesign. Its approved layout retains subscription/nodes on the left and traffic/account on the right, white panels, quiet neutral borders, black commands and green status. Its local workspace maximum remains 1136px and utility header 64px; portal typography, responsive records and spacing follow existing `.edu-app.edu-clear` source. Light is the portal default and `cpanel.portal.theme` remains independent; the dark variant retains its own palette.

The former public styles have been consolidated, in cascade order, into `web/src/public-surfaces.css`, alongside the preserved `web/src/portal.css` base; this consolidation does not authorize a portal redesign. Public rules exclude `html[data-admin-theme]`. Corresponding components remain `PortalApp.tsx` and `PortalDashboardPage.tsx`.

Portal subscription/node links, usage, account values, copy/import, password change and reset preserve existing API behavior. Delegated sessions visibly permit copying only and hide import/reset/password controls. Existing dialog focus/Escape/scroll locking remain. No payment or new server permission claim is introduced.

## Current evidence and limits

The final 1440px light portal capture is pixel-identical to the T00 baseline. Real local delegated access was verified with synthetic data and a separate PostgreSQL test backend. These checks do not constitute production deployment or exhaustive server authorization verification. Administration details and final scoped review evidence are under `design-system/admin-console-2026-11-03/`. No shipping raster assets.
