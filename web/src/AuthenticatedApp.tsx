import { Redirect, Route, Switch } from "wouter";
import { demoMode } from "./auth";
import { AdminNavigation } from "./components/AdminNavigation";
import { AdminThemeProvider } from "./components/AdminTheme";
import "./admin-console.css";
import { AppShell } from "./components/AppShell";
import { EmptyState, ToastProvider } from "./components/ui";
import {
  AccessGroupsPage,
  MachinesPage,
  OutboundsPage,
  PlansPage,
  RoutesPage,
  UsersPage,
} from "./pages/ResourcePages";
import { AuditPage } from "./pages/AuditPage";
import { HistoryPage } from "./pages/HistoryPage";
import { NodeEditorPage } from "./pages/NodeEditorPage";
import { NodesPage } from "./pages/NodesPage";
import { OverviewPage } from "./pages/OverviewPage";
import { SettingsPage, SubscriptionPage } from "./pages/SettingsPage";
import { UserAccessIPsPage } from "./pages/UserAccessIPsPage";
import type { Admin, Session } from "./types";

export default function AuthenticatedApp({
  session,
  onAdminChange,
  onLogout,
}: {
  session: Session;
  onAdminChange: (admin: Admin) => void;
  onLogout: () => void;
}) {
  return (
    <AdminThemeProvider><AdminNavigation>
      <ToastProvider>
        <AppShell admin={session.admin} demo={demoMode} onLogout={onLogout}>
          <Switch>
            <Route path="/" component={OverviewPage} />
            <Route path="/machines" component={MachinesPage} />
            <Route path="/machines/new" component={MachinesPage} />
            <Route path="/machines/:id/edit" component={MachinesPage} />
            <Route path="/nodes" component={NodesPage} />
            <Route path="/nodes/new" component={NodeEditorPage} />
            <Route path="/nodes/:id/edit" component={NodeEditorPage} />
            <Route path="/access-groups" component={AccessGroupsPage} />
            <Route path="/access-groups/new" component={AccessGroupsPage} />
            <Route path="/access-groups/:id/edit" component={AccessGroupsPage} />
            <Route path="/plans" component={PlansPage} />
            <Route path="/plans/new" component={PlansPage} />
            <Route path="/plans/:id/edit" component={PlansPage} />
            <Route path="/users" component={UsersPage} />
            <Route path="/users/new" component={UsersPage} />
            <Route path="/users/:id/edit" component={UsersPage} />
            <Route path="/subscription" component={SubscriptionPage} />
            <Route path="/access-ips" component={UserAccessIPsPage} />
            <Route path="/routes" component={RoutesPage} />
            <Route path="/routes/new" component={RoutesPage} />
            <Route path="/routes/:id/edit" component={RoutesPage} />
            <Route path="/outbounds" component={OutboundsPage} />
            <Route path="/outbounds/new" component={OutboundsPage} />
            <Route path="/outbounds/:id/edit" component={OutboundsPage} />
            <Route path="/history" component={HistoryPage} />
            <Route path="/settings">
              <SettingsPage admin={session.admin} onAdminChange={onAdminChange} />
            </Route>
            <Route path="/audit" component={AuditPage} />
            <Route path="/404">
              <div className="page">
                <EmptyState title="页面不存在" description="该地址没有对应的管理页面。" />
              </div>
            </Route>
            <Route>
              <Redirect to="/404" replace />
            </Route>
          </Switch>
        </AppShell>
      </ToastProvider>
    </AdminNavigation></AdminThemeProvider>
  );
}
