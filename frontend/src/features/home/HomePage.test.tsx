import "@testing-library/jest-dom/vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { HomePage } from "./HomePage";

function renderHomePage() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false }
    }
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <HomePage />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe("HomePage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(window, "fetch").mockImplementation((input) => {
      const url = String(input);
      if (url.endsWith("/api/v1/me")) {
        return Promise.resolve(
          Response.json({
            ok: true,
            data: { user: { id: "u1", displayName: "TomyJan", avatarUrl: "", qrImageUrl: "/api/v1/user/qr?userId=u1" } }
          })
        );
      }
      if (url.includes("/api/v1/group/list")) {
        return Promise.resolve(Response.json({ ok: true, data: { groups: [] } }));
      }
      return Promise.resolve(Response.json({ ok: true, data: {} }));
    });
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  test("uses the shared user navigation and keeps profile menu focused on account actions", async () => {
    renderHomePage();

    await waitFor(() => expect(screen.getByText("BWS 互助")).toBeInTheDocument());
    expect(screen.getByRole("link", { name: "互助组" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "个人中心" })).toBeInTheDocument();
    const userMenuButton = await screen.findByRole("button", { name: /TomyJan/ });

    fireEvent.click(userMenuButton);
    expect(screen.getByRole("menuitem", { name: "TomyJan" })).toBeInTheDocument();
    expect(screen.queryByText(/ID:/)).not.toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "个人中心" })).not.toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "退出登录" })).toBeInTheDocument();
  });

  test("keeps group actions in the page body", async () => {
    renderHomePage();

    await waitFor(() => expect(screen.getByText("BWS 互助")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button", { name: "创建或加入互助组" }));
    expect(screen.getByRole("menuitem", { name: "创建互助组" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "加入互助组" })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "更新二维码" })).not.toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "删除二维码" })).not.toBeInTheDocument();
  });
});
