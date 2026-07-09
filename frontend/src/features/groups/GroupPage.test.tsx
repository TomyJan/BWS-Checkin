import "@testing-library/jest-dom/vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { GroupPage } from "./GroupPage";

const testDir = dirname(fileURLToPath(import.meta.url));

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
                  imageUrl: "https://i0.hdslb.com/bfs/activity/rainbow.png",
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
                  imageUrl: "https://i0.hdslb.com/bfs/activity/stage.png",
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
    expect(screen.getByRole("img", { name: "彩虹补给站 图标" })).toHaveAttribute(
      "src",
      "/api/v1/task/image?taskId=t1"
    );
    expect(screen.getByRole("button", { name: "切换点位：彩虹补给站" })).toBeInTheDocument();
    expect(screen.getByText("场馆打卡 · 乐园币 x3")).toBeInTheDocument();
    expect(screen.queryByText(/点击切换点位/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /彩虹补给站/ }));
    expect(screen.getByText("选择点位")).toBeInTheDocument();
    expect(screen.getByRole("tablist", { name: "点位分组" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "场馆打卡" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "舞台任务" })).toBeInTheDocument();
    expect(screen.getByTestId("task-icon-t1").querySelector("img")).toHaveAttribute("src", "/api/v1/task/image?taskId=t1");
    expect(screen.queryByText("完成彩虹补给站互动")).not.toBeInTheDocument();
    expect(screen.getByText("乐园币 x3")).toBeInTheDocument();
    expect(screen.getByText("在彩虹补给站完成互动并出示二维码。")).toBeInTheDocument();
    expect(screen.queryByText("完成主舞台应援")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "舞台任务" }));
    expect(screen.getByTestId("task-icon-t2")).toBeInTheDocument();
    expect(screen.queryByText("完成主舞台应援")).not.toBeInTheDocument();
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
    expect(await screen.findByRole("alert")).toHaveTextContent("乐园任务已同步");
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

  test("keeps task picker compact without stretching sparse task cards", () => {
    const css = readFileSync(resolve(testDir, "../../styles.css"), "utf8");

    expect(css).toMatch(/\.task-sheet\s*\{[\s\S]*max-width:\s*960px;/);
    expect(css).toMatch(/\.task-picker-container\s*\{[\s\S]*align-items:\s*flex-end !important;/);
    expect(css).toMatch(/\.task-picker-paper\s*\{[\s\S]*height:\s*min\(620px, calc\(100vh - 48px\)\) !important;/);
    expect(css).toMatch(/\.task-picker-content\s*\{[\s\S]*overflow-y:\s*auto;/);
    expect(css).toMatch(/\.task-picker-list\s*\{[\s\S]*display:\s*flex !important;/);
    expect(css).toMatch(/\.task-picker-list\s*\{[\s\S]*flex-direction:\s*column;/);
    expect(css).toMatch(/\.task-picker-list\s*\{[\s\S]*justify-content:\s*flex-start;/);
    expect(css).toMatch(/\.task-picker-list\s*\{[\s\S]*flex:\s*0 0 auto;/);
    expect(css).not.toMatch(/\.task-picker-list\s*\{[\s\S]*flex:\s*1 1 auto;/);
    expect(css).toMatch(/\.task-picker-list\s*\{[\s\S]*overflow:\s*visible;/);
    expect(css).toMatch(/\.task-picker-card\s*\{[\s\S]*height:\s*126px !important;/);
    expect(css).toMatch(/\.task-picker-card\s*\{[\s\S]*min-height:\s*126px !important;/);
    expect(css).toMatch(/\.task-picker-card\s*\{[\s\S]*max-height:\s*126px !important;/);
    expect(css).toMatch(/\.task-picker-card\s*\{[\s\S]*flex:\s*0 0 126px !important;/);
    expect(css).toMatch(/\.task-picker-card\s*\{[\s\S]*flex-grow:\s*0 !important;/);
    expect(css).toMatch(/\.task-picker-card\s*\{[\s\S]*flex-shrink:\s*0 !important;/);
    expect(css).toMatch(/\.task-picker-card\s*\{[\s\S]*overflow:\s*hidden;/);
    expect(css).toMatch(/\.task-picker-tabs-wrap \.MuiTabs-scroller\s*\{[\s\S]*overflow-x:\s*auto !important;/);
    expect(css).toMatch(/\.task-picker-tabs-wrap \.MuiTab-root\s*\{[\s\S]*padding:\s*0 4px !important;/);
  });
});
