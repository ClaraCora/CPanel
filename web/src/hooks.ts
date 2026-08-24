import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "./api";

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
