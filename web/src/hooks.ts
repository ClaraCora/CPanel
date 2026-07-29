import { useCallback, useEffect, useState } from "react";
import { api } from "./api";

export function useResource<T>(path: string, initial: T) {
  const [data, setData] = useState<T>(initial);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const reload = useCallback(async () => {
    setLoading(true);
    setError("");
    try { setData(await api.get<T>(path)); }
    catch (reason) { setError(reason instanceof Error ? reason.message : "加载失败，请稍后重试"); }
    finally { setLoading(false); }
  }, [path]);
  useEffect(() => { void reload(); }, [reload]);
  return { data, setData, loading, error, reload };
}
