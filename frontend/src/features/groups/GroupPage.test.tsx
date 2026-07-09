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
                  name: "彩虹补给站",
                  sortOrder: 1,
                  completedCount: 0,
                  totalCount: 1,
                  members: [
                    {
                      member: { id: "u1", displayName: "Alice", qrImageUrl: "/api/v1/user/qr?userId=u1" },
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

    fireEvent.click(screen.getByRole("button", { name: /彩虹补给站/ }));
    expect(screen.getByText("选择点位")).toBeInTheDocument();
    expect(screen.getByText("BW2026 周五")).toBeInTheDocument();
  });
});
