const sensitiveKey = /password|passwd|secret|token|private.?key|certificate|cert_pem|key_pem/i;

export function redactRevision(value: unknown, path = ""): unknown {
  if (sensitiveKey.test(path.split(".").at(-1) ?? "")) return value ? "[已隐藏]" : value;
  if (Array.isArray(value)) return value.map((item) => redactRevision(item, path));
  if (value && typeof value === "object") return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, redactRevision(item, path ? `${path}.${key}` : key)]));
  return value;
}

function flatten(value: unknown, path: string, result: Map<string, unknown>) {
  if (value && typeof value === "object" && !Array.isArray(value) && Object.keys(value).length) {
    Object.entries(value).forEach(([key, item]) => flatten(item, path ? `${path}.${key}` : key, result));
  } else result.set(path || "配置", value);
}

export function revisionDiff(before: unknown, after: unknown) {
  const previous = new Map<string, unknown>(), next = new Map<string, unknown>();
  flatten(before, "", previous); flatten(after, "", next);
  return [...new Set([...previous.keys(), ...next.keys()])].sort().filter((path) => JSON.stringify(previous.get(path)) !== JSON.stringify(next.get(path))).map((path) => ({
    path,
    before: sensitiveKey.test(path) && previous.has(path) ? "[已隐藏]" : display(previous.get(path)),
    after: sensitiveKey.test(path) && next.has(path) ? "[已隐藏]" : display(next.get(path)),
  }));
}

function display(value: unknown) {
  if (value === undefined) return "—";
  if (typeof value === "string") return value;
  return JSON.stringify(redactRevision(value));
}
