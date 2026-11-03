import { createContext, useCallback, useContext, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { Router, type RouterProps } from "wouter";
import { rememberListPosition, withDemo } from "../navigation";
import { ConfirmDialog } from "./ui";

type Guard = { dirty: boolean; busy: boolean };
type NavigationContextValue = {
  register: (key: symbol, guard: Guard) => () => void;
  request: (action: () => void) => void;
  bypass: (action: () => void) => void;
};
const NavigationContext = createContext<NavigationContextValue>({ register: () => () => undefined, request: (action) => action(), bypass: (action) => action() });
const entryKey = "__cpanelEntry";
const currentURL = () => location.pathname + location.search + location.hash;
const stateObject = () => history.state && typeof history.state === "object" ? history.state as Record<string, unknown> : {};

/** Keep wouter and its authenticated loading boundary; guard push/replace and restore blocked POPs before unmounting. */
export function AdminNavigation({ children }: { children: ReactNode }) {
  const guards = useRef(new Map<symbol, Guard>());
  const pending = useRef<(() => void) | null>(null);
  const bypassed = useRef(false);
  const position = useRef({ index: 0, url: currentURL() });
  const restoring = useRef<(() => void) | null>(null);
  const allowedPop = useRef(false);
  const [confirm, setConfirm] = useState(false);
  const [notice, setNotice] = useState("");
  const blocked = () => [...guards.current.values()].some((guard) => guard.dirty || guard.busy);
  const busy = () => [...guards.current.values()].some((guard) => guard.busy);

  const register = useCallback((key: symbol, guard: Guard) => {
    guards.current.set(key, guard);
    return () => { guards.current.delete(key); };
  }, []);
  const bypass = useCallback((action: () => void) => {
    bypassed.current = true;
    try { action(); } finally { bypassed.current = false; }
  }, []);
  const request = useCallback((action: () => void) => {
    if (bypassed.current || !blocked()) { action(); return; }
    if (busy()) { setNotice("正在保存，请等待完成后再离开。"); return; }
    pending.current = action;
    setConfirm(true);
  }, []);

  useLayoutEffect(() => {
    const existing = stateObject()[entryKey];
    position.current = { index: typeof existing === "number" ? existing : 0, url: currentURL() };
    history.replaceState({ ...stateObject(), [entryKey]: position.current.index }, "", currentURL());
    function sync() {
      const index = stateObject()[entryKey];
      position.current = { index: typeof index === "number" ? index : position.current.index, url: currentURL() };
    }
    function onPop(event: PopStateEvent) {
      const incoming = typeof event.state?.[entryKey] === "number" ? event.state[entryKey] as number : position.current.index - 1;
      if (restoring.current) {
        event.stopImmediatePropagation();
        const afterRestore = restoring.current;
        restoring.current = null;
        afterRestore();
        return;
      }
      if (allowedPop.current || !blocked()) {
        allowedPop.current = false;
        position.current = { index: incoming, url: currentURL() };
        return;
      }
      event.stopImmediatePropagation();
      const delta = incoming - position.current.index;
      if (!delta) {
        history.replaceState({ ...stateObject(), [entryKey]: position.current.index }, "", position.current.url);
        return;
      }
      restoring.current = () => request(() => { allowedPop.current = true; history.go(delta); });
      history.go(-delta);
    }
    function beforeUnload(event: BeforeUnloadEvent) {
      if (!blocked()) return;
      event.preventDefault();
      event.returnValue = "";
    }
    window.addEventListener("popstate", onPop, true);
    window.addEventListener("pushState", sync);
    window.addEventListener("replaceState", sync);
    window.addEventListener("beforeunload", beforeUnload);
    return () => {
      window.removeEventListener("popstate", onPop, true);
      window.removeEventListener("pushState", sync);
      window.removeEventListener("replaceState", sync);
      window.removeEventListener("beforeunload", beforeUnload);
    };
  }, [request]);

  const aroundNav: NonNullable<RouterProps["aroundNav"]> = (navigate, target, options) => {
    const destination = withDemo(target);
    const action = () => {
      rememberListPosition(destination);
      const index = position.current.index + (options?.replace ? 0 : 1);
      navigate(destination, { ...options, state: { ...options?.state, [entryKey]: index } });
    };
    if (new URL(destination, location.origin).pathname === location.pathname && new URL(destination, location.origin).searchParams.get("copy") === new URLSearchParams(location.search).get("copy")) action();
    else request(action);
  };

  return <NavigationContext.Provider value={{ register, request, bypass }}><Router aroundNav={aroundNav} hrefs={withDemo}>{children}
    {notice && <div className="navigation-notice" role="status"><span>{notice}</span><button type="button" onClick={() => setNotice("")}>关闭</button></div>}
    <ConfirmDialog open={confirm} title="放弃未保存修改？" description="当前页面有未保存的修改。放弃后将离开此页面。" confirmLabel="放弃修改并离开" cancelLabel="继续编辑" onClose={() => { pending.current = null; setConfirm(false); }} onConfirm={() => { const action = pending.current; pending.current = null; setConfirm(false); if (action) bypass(action); }} />
  </Router></NavigationContext.Provider>;
}

export function useUnsavedChanges(dirty: boolean, busy = false) {
  const { register, bypass } = useContext(NavigationContext);
  const key = useRef(Symbol("form"));
  useLayoutEffect(() => register(key.current, { dirty, busy }), [busy, dirty, register]);
  return bypass;
}

export function useGuardedAction() { return useContext(NavigationContext).request; }
