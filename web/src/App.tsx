import { Component, Suspense, lazy, useEffect, useState, type ReactNode } from "react";
import { adminSessionExpiredEvent, authApi, demoMode } from "./auth";
import { ToastProvider } from "./components/ui";
import { ThemeToggle } from "./components/ThemeToggle";
import { LoginPage } from "./pages/LoginPage";
import type { Session } from "./types";

const AuthenticatedApp = lazy(() => import("./AuthenticatedApp"));

export default function App() {
  const [session, setSession] = useState<Session | null>(null);
  const [checking, setChecking] = useState(!demoMode);
  const [sessionNotice, setSessionNotice] = useState("");
  const [reauthenticating, setReauthenticating] = useState(false);
  const [editorGeneration, setEditorGeneration] = useState(0);

  useEffect(() => {
    if (demoMode) return;
    authApi
      .current()
      .then(setSession)
      .catch(() => setSession(null))
      .finally(() => setChecking(false));
  }, []);

  useEffect(() => {
    const expireSession = () => {
      setReauthenticating(true);
      setSessionNotice("登录状态已过期。使用原管理员账号重新登录后继续编辑；当前输入仅保留在本页内存中，请勿刷新或关闭页面。");
    };
    window.addEventListener(adminSessionExpiredEvent, expireSession);
    return () => window.removeEventListener(adminSessionExpiredEvent, expireSession);
  }, []);

  function loggedIn(value: Session) {
    // Another administrator must never inherit the previous administrator's draft.
    if (session && value.admin.id !== session.admin.id) {
      setEditorGeneration((current) => current + 1);
      window.history.replaceState(null, "", "/");
    }
    setSessionNotice("");
    setReauthenticating(false);
    setSession(value);
  }

  async function logout() {
    try {
      await authApi.logout();
    } finally {
      setSession(null);
      window.location.reload();
    }
  }

  if (checking) return <BootScreen label="正在验证管理会话…" />;
  if (!session)
    return (
      <ToastProvider>
        <LoginPage initialError={sessionNotice} onLogin={loggedIn} />
      </ToastProvider>
    );

  return <>
    {reauthenticating && <ToastProvider><LoginPage initialError={sessionNotice} onLogin={loggedIn} /></ToastProvider>}
    <div hidden={reauthenticating} inert={reauthenticating} style={reauthenticating ? { display: "none" } : undefined}>
    <AuthenticatedLoadBoundary key={editorGeneration}>
      <Suspense fallback={<BootScreen label="正在加载管理后台…" />}>
        <AuthenticatedApp
          session={session}
          onAdminChange={(admin) => setSession((current) => current ? { ...current, admin } : current)}
          onLogout={() => void logout()}
        />
      </Suspense>
    </AuthenticatedLoadBoundary>
    </div>
  </>;
}

function BootScreen({ label, children }: { label: string; children?: ReactNode }) {
  return <main className="admin-entry boot-screen">
    <ThemeToggle className="theme-toggle--floating" />
    <div className="brand__mark">C</div>
    <span>{label}</span>
    {children}
  </main>;
}

class AuthenticatedLoadBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  render() {
    if (this.state.failed) {
      return <BootScreen label="管理后台加载失败">
        <button type="button" className="button button--secondary" onClick={() => window.location.reload()}>重新加载</button>
      </BootScreen>;
    }
    return this.props.children;
  }
}
