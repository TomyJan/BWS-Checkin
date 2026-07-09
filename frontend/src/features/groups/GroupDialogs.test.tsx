import "@testing-library/jest-dom/vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { CreateGroupDialog } from "./GroupDialogs";

function renderWithQueryClient(node: React.ReactElement) {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false }
    }
  });
  return render(<QueryClientProvider client={client}>{node}</QueryClientProvider>);
}

describe("GroupDialogs", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  test("shows business error messages when group creation fails", async () => {
    vi.spyOn(window, "fetch").mockResolvedValue(
      Response.json({
        ok: false,
        error: { code: "group_id_conflict", message: "组 ID 已存在" }
      })
    );

    renderWithQueryClient(<CreateGroupDialog open onClose={vi.fn()} onDone={vi.fn()} />);

    fireEvent.change(screen.getByLabelText("名称"), { target: { value: "BW2026 周五" } });
    fireEvent.change(screen.getByLabelText("ID / 邀请码"), { target: { value: "bw2026-fri" } });
    fireEvent.click(screen.getByRole("button", { name: "创建" }));

    await waitFor(() => expect(screen.getByText("组 ID 已存在")).toBeInTheDocument());
  });

  test("uses actual BWS event dates when creating a group", () => {
    renderWithQueryClient(<CreateGroupDialog open onClose={vi.fn()} onDone={vi.fn()} />);

    expect(screen.getByRole("button", { name: "7 月 10 日" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "7 月 11 日" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "7 月 12 日" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "周五" })).not.toBeInTheDocument();
  });
});
