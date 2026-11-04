import { expect, it } from "vitest";
import { demoNodes } from "./mock";
import { demoNodeRevisions, recordDemoNodeRevision, restoreDemoNodeRevision } from "./demoNodeHistory";

it("keeps history immutable through draft edits, restoration and subsequent publication", () => {
  const node = structuredClone({ ...demoNodes[0], id: "history-demo-test" });
  const first = demoNodeRevisions(node)[0];
  node.config.transport = "ws";
  node.endpoints[0].host = "changed.example.net";
  node.has_draft = true;
  expect(demoNodeRevisions(node)[0]).toEqual(first);
  const restored = restoreDemoNodeRevision(node, first.revision)!;
  expect(restored.current_revision).toBe(node.current_revision);
  expect(restored.applied_revision).toBe(node.applied_revision);
  expect(restored.status).toBe(node.status);
  expect(restored.has_draft).toBe(true);
  expect(restored.endpoints[0].host).toBe(first.snapshot!.node.endpoints[0].host);
  expect(restored.route_policy_id).toBeNull();
  expect(restoreDemoNodeRevision(node, first.revision - 1)).toBeUndefined();
  restored.current_revision += 1;
  restored.has_draft = false;
  recordDemoNodeRevision(restored);
  const history = demoNodeRevisions(restored);
  expect(history[0].revision).toBe(first.revision + 1);
  expect(history[1]).toEqual(first);
  history[0].snapshot!.node.name = "mutated response";
  expect(demoNodeRevisions(restored)[0].snapshot!.node.name).toBe(node.name);
});
