import { createContext, useContext, useLayoutEffect, useState, type ReactNode } from "react";
import { useLocation } from "wouter";
import { Moon, Sun } from "lucide-react";
import type { Theme } from "../theme";

const AdminThemeContext = createContext<{ theme: Theme; toggle: () => void }>({ theme: "light", toggle: () => undefined });

function readPreference(): Theme | null {
  try {
    const value = localStorage.getItem("cpanel.theme");
    return value === "dark" || value === "light" ? value : null;
  } catch {
    return null;
  }
}

export function AdminThemeProvider({ children }: { children: ReactNode }) {
  const [location] = useLocation();
  const path = location.replace(/\/+$/, "") || "/";
  const layout = path === "/plans" ? "B"
    : path === "/" || path === "/settings" || /^\/(users|plans|outbounds)\/(new|[^/]+\/edit)$/.test(path) ? "C" : "A";
  const [preference, setPreference] = useState(readPreference);
  const theme = preference ?? (layout === "B" ? "dark" : "light");
  useLayoutEffect(() => {
    document.documentElement.dataset.consoleLayout = layout;
    return () => { delete document.documentElement.dataset.consoleLayout; };
  }, [layout]);
  useLayoutEffect(() => {
    const previous = document.documentElement.style.colorScheme;
    document.documentElement.dataset.adminTheme = theme;
    document.documentElement.style.colorScheme = theme;
    return () => { delete document.documentElement.dataset.adminTheme; document.documentElement.style.colorScheme = previous; };
  }, [theme]);
  function toggle() {
    const next = theme === "dark" ? "light" : "dark";
    try { localStorage.setItem("cpanel.theme", next); } catch { /* Preferences are optional. */ }
    setPreference(next);
  }
  return <AdminThemeContext.Provider value={{ theme, toggle }}><div className="admin-console" data-admin-theme={theme}>{children}<div id="admin-overlays" /></div></AdminThemeContext.Provider>;
}

export function AdminThemeToggle() {
  const { theme, toggle } = useContext(AdminThemeContext);
  const label = theme === "dark" ? "切换为浅色模式" : "切换为深色模式";
  return <button type="button" className="icon-button" aria-label={label} title={label} aria-pressed={theme === "dark"} onClick={toggle}>{theme === "dark" ? <Sun size={18} aria-hidden="true" /> : <Moon size={18} aria-hidden="true" />}</button>;
}
