import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "./api";
import type { PageResult } from "./types";

type InFlightRequest = {
  controller: AbortController;
  promise: Promise<void>;
  sequence: number;
};

function isAbortError(reason: unknown) {
  return reason instanceof DOMException && reason.name === "AbortError";
}

export function useResource<T>(path: string, initial: T, refreshIntervalMs = 0) {
  const [data, setData] = useState<T>(initial);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");
  const [refreshError, setRefreshError] = useState("");
  const [lastSuccessAt, setLastSuccessAt] = useState<number | null>(null);
  const sequenceRef = useRef(0);
  const inFlightRef = useRef<InFlightRequest | null>(null);

  const reload = useCallback((silent = false): Promise<void> => {
    const current = inFlightRef.current;
    if (silent && current) return current.promise;
    current?.controller.abort();

    const controller = new AbortController();
    const sequence = ++sequenceRef.current;
    if (silent) setRefreshing(true);
    else {
      setRefreshing(false);
      setLoading(true);
      setError("");
      setRefreshError("");
    }

    let request: InFlightRequest | null = null;
    const promise = (async () => {
      try {
        const next = await api.get<T>(path, controller.signal);
        if (sequence !== sequenceRef.current) return;
        setData(next);
        setError("");
        setRefreshError("");
        setLastSuccessAt(Date.now());
      } catch (reason) {
        if (isAbortError(reason) || sequence !== sequenceRef.current) return;
        const message = reason instanceof Error ? reason.message : "加载失败，请稍后重试";
        if (silent) setRefreshError(message);
        else setError(message);
      } finally {
        if (sequence !== sequenceRef.current) return;
        if (request && inFlightRef.current === request) inFlightRef.current = null;
        if (silent) setRefreshing(false);
        else setLoading(false);
      }
    })();
    request = { controller, promise, sequence };
    inFlightRef.current = request;
    return promise;
  }, [path]);

  useEffect(() => {
    void reload();
    return () => {
      sequenceRef.current += 1;
      inFlightRef.current?.controller.abort();
      inFlightRef.current = null;
    };
  }, [reload]);

  useEffect(() => {
    if (refreshIntervalMs <= 0) return;
    const timer = window.setInterval(() => void reload(true), refreshIntervalMs);
    return () => window.clearInterval(timer);
  }, [refreshIntervalMs, reload]);

  return {
    data,
    setData,
    loading,
    refreshing,
    error,
    refreshError,
    lastSuccessAt,
    stale: Boolean(refreshError && lastSuccessAt),
    reload,
  };
}

function readPageNumber(value: string | null, fallback: number) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

export function usePagedResource<T>(path: string, initial: T[] = []) {
  const initialURL = typeof window === "undefined" ? null : new URLSearchParams(window.location.search);
  const [page, setPageState] = useState(() => readPageNumber(initialURL?.get("page") ?? null, 1));
  const [pageSize] = useState(() => Math.min(100, Math.max(1, readPageNumber(initialURL?.get("page_size") ?? null, 50))));
  const [query, setQueryState] = useState(() => initialURL?.get("q") ?? "");
  const [data, setData] = useState<T[]>(initial);
  const [total, setTotal] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const request = useRef<AbortController | null>(null);

  const setPage = useCallback((value: number) => setPageState(Math.max(1, value)), []);
  const setQuery = useCallback((value: string) => { setQueryState(value); setPageState(1); }, []);
  const reload = useCallback(async () => {
    request.current?.abort();
    const controller = new AbortController(); request.current = controller;
    setLoading(true); setError("");
    const params = new URLSearchParams({ page: String(page), page_size: String(pageSize) });
    if (query.trim()) params.set("q", query.trim());
    try {
      const result = await api.get<PageResult<T> | T[]>(`${path}?${params.toString()}`, controller.signal);
      if (Array.isArray(result)) { setData(result); setTotal(result.length); setHasMore(false); }
      else { setData(result.items ?? []); setTotal(result.total ?? 0); setHasMore(Boolean(result.has_more)); }
    } catch (reason) {
      if (reason instanceof DOMException && reason.name === "AbortError") return;
      setError(reason instanceof Error ? reason.message : "加载失败，请稍后重试");
    } finally { if (request.current === controller) { request.current = null; setLoading(false); } }
  }, [page, pageSize, path, query]);

  useEffect(() => { void reload(); return () => request.current?.abort(); }, [reload]);
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    params.set("page", String(page)); params.set("page_size", String(pageSize));
    if (query.trim()) params.set("q", query.trim()); else params.delete("q");
    window.history.replaceState({}, "", `${window.location.pathname}?${params.toString()}`);
  }, [page, pageSize, query]);
  return { data, page, pageSize, query, total, hasMore, loading, error, setPage, setQuery, reload };
}
