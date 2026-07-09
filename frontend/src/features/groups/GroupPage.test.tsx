import "@testing-library/jest-dom/vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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
    user: { id: "owner", displayName: "Owner", avatarUrl: "", qrImageUrl: "" }
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
  beforeEach(() => {
    vi.restoreAllMocks();
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
                      completed: false,
                      completedAt: null,
                      updatedAt: null,
                      checkedById: null,
                      checkedByName: ""
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
                      checkedByName: ""
                    }
                  ]
                }
              ]
            }
          })
        );
      }
      return Promise.resolve(Response.json({ ok: true, data: {} }));
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  test("does not render duplicated QR subtitle and ignores control clicks for overlay toggle", async () => {
    renderGroupPage();

    await waitFor(() => expect(screen.getByText("BW2026 周五")).toBeInTheDocument());
    expect(screen.queryByText(/正在查看/)).not.toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Alice 的二维码" })).toHaveAttribute("src", "/api/v1/user/qr?userId=u1");
    expect(screen.getByTestId("current-task-icon")).toBeInTheDocument();
    expect(screen.getByText("场馆打卡 · 乐园币 x3 · 点击切换点位")).toBeInTheDocument();

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
});
