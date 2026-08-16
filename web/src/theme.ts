import { useEffect, useState } from "react";

export type Theme = "dark" | "light";

const storageKey = "cpanel.theme";
const changeEvent = "cpanel:theme-change";

function fallbackTheme(): Theme {
  return window.location.pathname === "/edu" || window.location.pathname === "/edu/" ? "light" : "dark";
}

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
  return applied === "dark" || applied === "light" ? applied : storedTheme() ?? fallbackTheme();
}

export function applyTheme(theme: Theme, persist = false) {
  document.documentElement.dataset.theme = theme;
  document.documentElement.style.colorScheme = theme;
  document.querySelector('meta[name="theme-color"]')?.setAttribute("content", theme === "dark" ? "#07090f" : "#f6f7f9");
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
  applyTheme(storedTheme() ?? fallbackTheme());
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
