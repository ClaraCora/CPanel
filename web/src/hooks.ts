import { useCallback, useEffect, useState } from "react";
import { api } from "./api";

export function useResource<T>(path: string, initial: T, refreshIntervalMs = 0) {
  const [data, setData] = useState<T>(initial);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const reload = useCallback(async (silent = false) => {
    if (!silent) {
      setLoading(true);
      setError("");
    }
    try { setData(await api.get<T>(path)); }
    catch (reason) { if (!silent) setError(reason instanceof Error ? reason.message : "加载失败，请稍后重试"); }
    finally { if (!silent) setLoading(false); }
  }, [path]);
  useEffect(() => { void reload(); }, [reload]);
  useEffect(() => {
    if (refreshIntervalMs <= 0) return;
    const timer = window.setInterval(() => void reload(true), refreshIntervalMs);
    return () => window.clearInterval(timer);
  }, [refreshIntervalMs, reload]);
  return { data, setData, loading, error, reload };
}
