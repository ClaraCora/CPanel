import { useEffect, useState } from "react";
import { Redirect, Route, Router, Switch } from "wouter";
import { api, demoMode, setCsrfToken } from "./api";
import { AppShell } from "./components/AppShell";
import { EmptyState, ToastProvider } from "./components/ui";
import { demoSession } from "./mock";
import {
  AccessGroupsPage,
  MachinesPage,
  OutboundsPage,
  PlansPage,
  RoutesPage,
  UsersPage,
} from "./pages/ResourcePages";
import { AuditPage } from "./pages/AuditPage";
import { LoginPage } from "./pages/LoginPage";
import { HistoryPage } from "./pages/HistoryPage";
import { NodeEditorPage } from "./pages/NodeEditorPage";
import { NodesPage } from "./pages/NodesPage";
import { OverviewPage } from "./pages/OverviewPage";
import { SettingsPage } from "./pages/SettingsPage";
import type { Session } from "./types";

export default function App() {
  const [session, setSession] = useState<Session | null>(
    demoMode ? demoSession : null,
  );
  const [checking, setChecking] = useState(!demoMode);

  useEffect(() => {
    if (demoMode) {
      setCsrfToken(demoSession.csrf_token);
      return;
    }
    api
      .get<Session>("/session")
      .then((value) => {
        setCsrfToken(value.csrf_token);
        setSession(value);
      })
      .catch(() => setSession(null))
      .finally(() => setChecking(false));
  }, []);

  async function logout() {
    try {
      await api.delete("/sessions/current");
    } finally {
      setSession(null);
    }
  }

  if (checking)
    return (
      <main className="boot-screen">
        <div className="brand__mark">C</div>
        <span>正在验证管理会话…</span>
      </main>
    );
  if (!session)
    return (
      <ToastProvider>
        <LoginPage onLogin={setSession} />
      </ToastProvider>
    );

  return (
    <Router>
      <ToastProvider>
        <AppShell
          admin={session.admin}
          demo={demoMode}
          onLogout={() => void logout()}
        >
          <Switch>
            <Route path="/" component={OverviewPage} />
            <Route path="/machines" component={MachinesPage} />
            <Route path="/nodes" component={NodesPage} />
            <Route path="/nodes/new" component={NodeEditorPage} />
            <Route path="/nodes/:id/edit" component={NodeEditorPage} />
            <Route path="/access-groups" component={AccessGroupsPage} />
            <Route path="/plans" component={PlansPage} />
            <Route path="/users" component={UsersPage} />
            <Route path="/routes" component={RoutesPage} />
            <Route path="/outbounds" component={OutboundsPage} />
            <Route path="/history" component={HistoryPage} />
            <Route path="/settings">
              <SettingsPage
                admin={session.admin}
                onAdminChange={(admin) => setSession((current) => current ? { ...current, admin } : current)}
              />
            </Route>
            <Route path="/audit" component={AuditPage} />
            <Route path="/404">
              <div className="page">
                <EmptyState
                  title="页面不存在"
                  description="该地址没有对应的管理页面。"
                />
              </div>
            </Route>
            <Route>
              <Redirect to="/404" replace />
            </Route>
          </Switch>
        </AppShell>
      </ToastProvider>
    </Router>
  );
}
