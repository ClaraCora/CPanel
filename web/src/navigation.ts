import { useEffect } from "react";
import { demoMode } from "./auth";

export const resourceLists = ["/machines", "/nodes", "/users", "/plans", "/access-groups", "/routes", "/outbounds"];

export function withDemo(path: string) {
  if (!demoMode) return path;
  const url = new URL(path, window.location.origin);
  if (url.origin !== window.location.origin) return path;
  url.searchParams.set("demo", "1");
  return `${url.pathname}${url.search}${url.hash}`;
}

export function safeReturnTo(base: string, value = new URLSearchParams(window.location.search).get("returnTo")) {
  try {
    const url = new URL(value || base, window.location.origin);
    if (url.origin === window.location.origin && url.pathname === base && !url.hash) return withDemo(url.pathname + url.search);
  } catch { /* Fall back to the resource's own list. */ }
  return withDemo(base);
}

export function editorHref(base: string, id = "new", copy?: string) {
  const query = new URLSearchParams({ returnTo: window.location.pathname === base ? window.location.pathname + window.location.search : withDemo(base) });
  if (copy) query.set("copy", copy);
  return withDemo(`${base}/${id === "new" ? "new" : `${encodeURIComponent(id)}/edit`}?${query}`);
}

type ListPosition = { top: number; resource: string | null };
const positions = new Map<string, ListPosition>();

export function rememberListPosition(destination: string) {
  if (!resourceLists.includes(window.location.pathname)) return;
  const target = new URL(destination, window.location.origin);
  if (!target.pathname.startsWith(window.location.pathname + "/")) return;
  positions.set(window.location.pathname + window.location.search, {
    top: window.scrollY,
    resource: document.activeElement instanceof HTMLElement ? document.activeElement.closest<HTMLElement>("[data-resource-id]")?.dataset.resourceId ?? null : null,
  });
}

export function useRestoreList(ready: boolean) {
  useEffect(() => {
    if (!ready) return;
    const key = window.location.pathname + window.location.search;
    const saved = positions.get(key);
    if (!saved) return;
    const frame = requestAnimationFrame(() => {
      window.scrollTo({ top: saved.top, behavior: "instant" });
      const target = saved.resource ? document.querySelector<HTMLElement>(`[data-resource-id="${CSS.escape(saved.resource)}"] :is(a,button)`) : null;
      (target ?? document.querySelector<HTMLElement>("main h1"))?.focus({ preventScroll: true });
      positions.delete(key);
    });
    return () => cancelAnimationFrame(frame);
  }, [ready]);
}
