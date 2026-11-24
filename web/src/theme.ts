import { useEffect, useState } from "react";

export type Theme = "dark" | "light";

export const isPortal = window.location.pathname === "/edu" || window.location.pathname === "/edu/";
const storageKey = isPortal ? "cpanel.portal.theme" : "cpanel.theme";
const changeEvent = "cpanel:theme-change";

function storedTheme(): Theme | null {
  try {
    const value = window.localStorage.getItem(storageKey);
    return value === "dark" || value === "light" ? value : null;
  } catch {
    return null;
  }
}

export function currentTheme(): Theme {
  const applied = document.documentElement.dataset.theme;
  return applied === "dark" || applied === "light" ? applied : storedTheme() ?? "light";
}

export function applyTheme(theme: Theme, persist = false) {
  document.documentElement.dataset.theme = theme;
  document.documentElement.style.colorScheme = theme;
  const surface = isPortal ? (theme === "dark" ? "#07090f" : "#f6f7f9")
    : getComputedStyle(document.documentElement).getPropertyValue("--surface").trim();
  if (surface) document.querySelector('meta[name="theme-color"]')?.setAttribute("content", surface);
  if (persist) {
    try {
      window.localStorage.setItem(storageKey, theme);
    } catch {
      // Theme persistence is optional when browser storage is unavailable.
    }
  }
  window.dispatchEvent(new CustomEvent<Theme>(changeEvent, { detail: theme }));
}

export function initializeTheme() {
  document.documentElement.toggleAttribute("data-admin-surface", !isPortal);
  applyTheme(storedTheme() ?? "light");
}

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(currentTheme);

  useEffect(() => {
    const sync = (event: Event) => setTheme((event as CustomEvent<Theme>).detail ?? currentTheme());
    window.addEventListener(changeEvent, sync);
    return () => window.removeEventListener(changeEvent, sync);
  }, []);

  return {
    theme,
    toggleTheme: () => applyTheme(theme === "dark" ? "light" : "dark", true),
  };
}
