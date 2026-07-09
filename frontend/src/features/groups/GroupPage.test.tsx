import "@testing-library/jest-dom/vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { GroupPage } from "./GroupPage";

function renderGroupPage() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false }
    }
  });
  client.setQueryData(["me"], {
    user: { id: "owner", displayName: "Owner", avatarUrl: "", qrImageUrl: "", qrSource: "uploaded" }
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={["/groups/g1"]}>
        <Routes>
          <Route path="/groups/:groupId" element={<GroupPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe("GroupPage", () => {
  let liveMode = false;
  let refreshCalls = 0;
  let taskSyncCalls = 0;

  beforeEach(() => {
    vi.restoreAllMocks();
    liveMode = false;
    refreshCalls = 0;
    taskSyncCalls = 0;
    Object.defineProperty(window.navigator, "onLine", {
      configurable: true,
      value: true
    });
    vi.spyOn(window, "fetch").mockImplementation((input) => {
      const url = String(input);
      if (url.includes("/api/v1/group/detail")) {
        return Promise.resolve(
          Response.json({
            ok: true,
            data: {
              group: {
                id: "g1",
                name: "BW2026 周五",
                day: "friday",
                description: "",
                role: "owner",
                memberCount: 1,
                taskCount: 1,
                joinLocked: false,
                archivedAt: null
              }
            }
          })
        );
      }
      if (url.includes("/api/v1/group/tasks")) {
        const memberStatus = liveMode
          ? {
              completed: true,
              completedAt: "2026-07-10T10:00:00Z",
              updatedAt: "2026-07-10T10:00:00Z",
              checkedById: "",
              checkedByName: "",
              status: "live_completed",
              source: "live",
              liveStale: false,
              liveCheckedAt: "2026-07-10T10:00:00Z",
              canToggle: false,
              canRefresh: true
            }
          : {
              completed: false,
              completedAt: null,
              updatedAt: null,
              checkedById: null,
              checkedByName: "",
              status: "manual_incomplete",
              source: "manual",
              liveStale: false,
              liveCheckedAt: null,
              canToggle: true,
              canRefresh: false
            };
        return Promise.resolve(
          Response.json({
            ok: true,
            data: {
              tasks: [
                {
                  id: "t1",
                  groupName: "场馆打卡",
                  name: "彩虹补给站",
                  title: "完成彩虹补给站互动",
                  rewardCoins: 3,
                  description: "在彩虹补给站完成互动并出示二维码。",
                  sortOrder: 1,
                  completedCount: 0,
                  totalCount: 1,
                  members: [
                    {
                      member: { id: "u1", displayName: "Alice", qrImageUrl: "/uploads/u1.png" },
                      ...memberStatus
                    }
                  ]
                },
                {
                  id: "t2",
                  groupName: "舞台任务",
                  name: "舞台应援任务",
                  title: "完成主舞台应援",
                  rewardCoins: 5,
                  description: "在主舞台完成应援任务并领取奖励。",
                  sortOrder: 2,
                  completedCount: 0,
                  totalCount: 1,
                  members: [
                    {
                      member: { id: "u1", displayName: "Alice", qrImageUrl: "/uploads/u1.png" },
                      completed: false,
                      completedAt: null,
                      updatedAt: null,
                      checkedById: null,
                      checkedByName: "",
                      status: "manual_incomplete",
                      source: "manual",
                      liveStale: false,
                      liveCheckedAt: null,
                      canToggle: true,
                      canRefresh: false
                    }
                  ]
                }
              ]
            }
          })
        );
      }
      if (url.includes("/api/v1/task/status/refresh")) {
        refreshCalls += 1;
        return Promise.resolve(Response.json({ ok: true, data: {} }));
      }
      if (url.includes("/api/v1/group/task/sync")) {
        taskSyncCalls += 1;
        return Promise.resolve(Response.json({ ok: true, data: { sync: { lastSuccessAt: "2026-07-10T12:00:00Z" } } }));
      }
      return Promise.resolve(Response.json({ ok: true, data: {} }));
    });
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  test("does not render duplicated QR subtitle and ignores control clicks for overlay toggle", async () => {
    renderGroupPage();

    await waitFor(() => expect(screen.getByText("BW2026 周五")).toBeInTheDocument());
    expect(screen.queryByText(/正在查看/)).not.toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Alice 的二维码" })).toHaveAttribute("src", "/api/v1/user/qr?userId=u1");
    expect(screen.getByTestId("current-task-icon")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "切换点位：彩虹补给站" })).toBeInTheDocument();
    expect(screen.getByText("场馆打卡 · 乐园币 x3")).toBeInTheDocument();
    expect(screen.queryByText(/点击切换点位/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /彩虹补给站/ }));
    expect(screen.getByText("选择点位")).toBeInTheDocument();
    expect(screen.getByRole("tablist", { name: "点位分组" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "场馆打卡" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "舞台任务" })).toBeInTheDocument();
    expect(screen.getByTestId("task-icon-t1")).toBeInTheDocument();
    expect(screen.getByText("完成彩虹补给站互动")).toBeInTheDocument();
    expect(screen.getByText("乐园币 x3")).toBeInTheDocument();
    expect(screen.getByText("在彩虹补给站完成互动并出示二维码。")).toBeInTheDocument();
    expect(screen.queryByText("完成主舞台应援")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "舞台任务" }));
    expect(screen.getByTestId("task-icon-t2")).toBeInTheDocument();
    expect(screen.getByText("完成主舞台应援")).toBeInTheDocument();
    expect(screen.getByText("乐园币 x5")).toBeInTheDocument();
    expect(screen.queryByText("完成彩虹补给站互动")).not.toBeInTheDocument();
    expect(screen.getByText("BW2026 周五")).toBeInTheDocument();
  });

  test("lets the owner sync BWS tasks from the management menu", async () => {
    renderGroupPage();

    await waitFor(() => expect(screen.getByText("BW2026 周五")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "互助组管理" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "同步乐园任务" }));

    await waitFor(() => expect(taskSyncCalls).toBe(1));
  });

  test("shows refresh action for Live status and disables it offline", async () => {
    liveMode = true;
    renderGroupPage();

    await waitFor(() => expect(screen.getByRole("button", { name: "刷新状态" })).toBeInTheDocument());
    expect(screen.queryByRole("button", { name: /撤销完成/ })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "刷新状态" }));
    await waitFor(() => expect(refreshCalls).toBe(1));
    expect(screen.getByText(/接口更新/)).toBeInTheDocument();

    cleanup();
    Object.defineProperty(window.navigator, "onLine", {
      configurable: true,
      value: false
    });
    renderGroupPage();

    await waitFor(() => expect(screen.getByRole("button", { name: "刷新状态" })).toBeDisabled());
  });
});
