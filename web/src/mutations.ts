/** A creation intent survives an uncertain response; changed input starts a new intent. */
export function creationIntent() {
  let previous = "";
  let key = "";
  return (payload: unknown) => {
    const next = JSON.stringify(payload);
    if (!key || next !== previous) {
      previous = next;
      key = crypto.randomUUID?.() ?? Array.from(crypto.getRandomValues(new Uint8Array(16)), (value) => value.toString(16).padStart(2, "0")).join("");
    }
    return key;
  };
}

export type MutationOptions = { idempotencyKey?: string; configVersion?: number };
export function mutationHeaders(options?: MutationOptions) {
  const headers = new Headers();
  if (options?.idempotencyKey) headers.set("Idempotency-Key", options.idempotencyKey);
  if (options?.configVersion !== undefined) headers.set("If-Match", `"${options.configVersion}"`);
  return headers;
}
