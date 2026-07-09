import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { pendingCompletionCount, queueCompletion } from "./completionSync";

describe("completionSync", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.spyOn(crypto, "randomUUID").mockReturnValue("pending-1");
  });

  afterEach(() => {
    vi.restoreAllMocks();
    localStorage.clear();
  });

  test("does not queue Live completion actions", () => {
    const queued = queueCompletion({
      groupId: "g1",
      taskId: "t1",
      userId: "u1",
      completed: true,
      updatedAt: "2026-07-10T10:00:00Z",
      source: "live",
      status: "live_completed"
    });

    expect(queued).toBe(false);
    expect(pendingCompletionCount()).toBe(0);
  });
});
