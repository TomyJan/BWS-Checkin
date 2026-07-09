import "@testing-library/jest-dom/vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { ProfilePage } from "./ProfilePage";

function renderProfilePage() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false }
    }
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={["/profile"]}>
        <ProfilePage />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe("ProfilePage", () => {
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
      return Promise.resolve(Response.json({ ok: true, data: {} }));
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  test("uses the shared user navigation and presents QR management as the main task", async () => {
    renderProfilePage();

    await waitFor(() => expect(screen.getByText("BWS 互助")).toBeInTheDocument());
    expect(screen.getByRole("link", { name: "互助组" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "个人中心" })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole("button", { name: /TomyJan/ })).toBeInTheDocument());
    expect(screen.getByRole("heading", { name: "个人中心" })).toBeInTheDocument();
    expect(await screen.findByRole("img", { name: "我的二维码" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "更新二维码" })).toBeInTheDocument();
  });
});
