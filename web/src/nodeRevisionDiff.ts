const sensitiveKey = /password|passwd|secret|token|private.?key|server.?key|certificate|cert_pem|key_pem|^pass$|^uuid$|^auth$|auth_str/i;

function sensitivePath(path: string) {
  const parts = path.split(".");
  return parts.some((part) => sensitiveKey.test(part)) || (parts.at(-1) === "id" && parts.includes("users"));
}

export function redactRevision(value: unknown, path = ""): unknown {
  if (sensitivePath(path)) return value ? "[已隐藏]" : value;
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
    before: sensitivePath(path) && previous.has(path) ? "[已隐藏]" : display(previous.get(path), path),
    after: sensitivePath(path) && next.has(path) ? "[已隐藏]" : display(next.get(path), path),
  }));
}

function display(value: unknown, path: string) {
  if (value === undefined) return "—";
  if (typeof value === "string") return value;
  return JSON.stringify(redactRevision(value, path));
}
