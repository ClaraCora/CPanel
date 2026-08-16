import { Component, Suspense, lazy, useEffect, useState, type ReactNode } from "react";
import { authApi, demoMode } from "./auth";
import { ToastProvider } from "./components/ui";
import { ThemeToggle } from "./components/ThemeToggle";
import { LoginPage } from "./pages/LoginPage";
import type { Session } from "./types";

const AuthenticatedApp = lazy(() => import("./AuthenticatedApp"));

export default function App() {
  const [session, setSession] = useState<Session | null>(null);
  const [checking, setChecking] = useState(!demoMode);

  useEffect(() => {
    if (demoMode) return;
    authApi
      .current()
      .then(setSession)
      .catch(() => setSession(null))
      .finally(() => setChecking(false));
  }, []);

  async function logout() {
    try {
      await authApi.logout();
    } finally {
      setSession(null);
      window.location.reload();
    }
  }

  if (checking)
    return (
      <main className="boot-screen">
        <ThemeToggle className="theme-toggle--floating" />
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
    <AuthenticatedLoadBoundary>
      <Suspense fallback={<BootScreen label="正在加载管理后台…" />}>
        <AuthenticatedApp
          session={session}
          onAdminChange={(admin) => setSession((current) => current ? { ...current, admin } : current)}
          onLogout={() => void logout()}
        />
      </Suspense>
    </AuthenticatedLoadBoundary>
  );
}

function BootScreen({ label }: { label: string }) {
  return <main className="boot-screen">
    <ThemeToggle className="theme-toggle--floating" />
    <div className="brand__mark">C</div>
    <span>{label}</span>
  </main>;
}

class AuthenticatedLoadBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  render() {
    if (this.state.failed) {
      return <main className="boot-screen">
        <ThemeToggle className="theme-toggle--floating" />
        <div className="brand__mark">C</div>
        <span>管理后台加载失败</span>
        <button type="button" className="button button--secondary" onClick={() => window.location.reload()}>重新加载</button>
      </main>;
    }
    return this.props.children;
  }
}
