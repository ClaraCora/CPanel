import { useLayoutEffect, type ReactNode } from "react";
import { useLocation } from "wouter";
import { useTheme } from "../theme";
export { ThemeToggle as AdminThemeToggle } from "./ThemeToggle";

export function AdminThemeProvider({ children }: { children: ReactNode }) {
  const [location] = useLocation();
  const path = location.replace(/\/+$/, "") || "/";
  const layout = path === "/plans" ? "B"
    : path === "/" || path === "/settings" || /^\/(users|plans|outbounds)\/(new|[^/]+\/edit)$/.test(path) ? "C" : "A";
  const { theme } = useTheme();

  useLayoutEffect(() => {
    document.documentElement.dataset.consoleLayout = layout;
    return () => { delete document.documentElement.dataset.consoleLayout; };
  }, [layout]);
  useLayoutEffect(() => {
    document.documentElement.dataset.adminTheme = theme;
    return () => { delete document.documentElement.dataset.adminTheme; };
  }, [theme]);
  return <div className="admin-console" data-admin-theme={theme}>{children}<div id="admin-overlays" /></div>;
}
