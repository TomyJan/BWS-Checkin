import "@testing-library/jest-dom/vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { AuthGate } from "./AuthGate";

function renderWithQueryClient(node: React.ReactElement) {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false }
    }
  });
  return render(<QueryClientProvider client={client}>{node}</QueryClientProvider>);
}

function setOnline(value: boolean) {
  Object.defineProperty(window.navigator, "onLine", {
    configurable: true,
    value
  });
}

describe("AuthGate", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
    window.history.pushState({}, "", "/");
    setOnline(true);
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  test("does not use cached user when the online session is rejected", async () => {
    localStorage.setItem(
      "bws:me",
      JSON.stringify({
        user: { id: "cached-user", displayName: "Cached", avatarUrl: "", qrImageUrl: "" }
      })
    );
    vi.spyOn(window, "fetch").mockImplementation((input) => {
      const url = String(input);
      if (url.endsWith("/api/v1/me")) {
        return Promise.resolve(new Response("", { status: 401 }));
      }
      if (url.endsWith("/api/v1/oauth/providers")) {
        return Promise.resolve(Response.json({ ok: true, data: { providers: [], devAuth: true } }));
      }
      return Promise.reject(new Error(`unexpected request ${url}`));
    });

    renderWithQueryClient(
      <AuthGate>
        <div>private content</div>
      </AuthGate>
    );

    await waitFor(() => expect(screen.getByRole("button", { name: "开发登录" })).toBeInTheDocument());
    expect(screen.queryByText("private content")).not.toBeInTheDocument();
    expect(localStorage.getItem("bws:me")).toBeNull();
  });

  test("uses cached user while offline", async () => {
    setOnline(false);
    localStorage.setItem(
      "bws:me",
      JSON.stringify({
        user: { id: "cached-user", displayName: "Cached", avatarUrl: "", qrImageUrl: "" }
      })
    );
    vi.spyOn(window, "fetch").mockRejectedValue(new TypeError("offline"));

    renderWithQueryClient(
      <AuthGate>
        <div>private content</div>
      </AuthGate>
    );

    await waitFor(() => expect(screen.getByText("private content")).toBeInTheDocument());
  });

  test("shows protected content after a successful dev login", async () => {
    vi.spyOn(window, "fetch").mockImplementation((input) => {
      const url = String(input);
      if (url.endsWith("/api/v1/me")) {
        return Promise.resolve(new Response("", { status: 401 }));
      }
      if (url.endsWith("/api/v1/oauth/providers")) {
        return Promise.resolve(Response.json({ ok: true, data: { providers: [], devAuth: true } }));
      }
      if (url.endsWith("/api/v1/dev/login?name=TomyJan")) {
        return Promise.resolve(
          Response.json({
            ok: true,
            data: { user: { id: "u1", displayName: "TomyJan", avatarUrl: "", qrImageUrl: "" } }
          })
        );
      }
      return Promise.reject(new Error(`unexpected request ${url}`));
    });

    renderWithQueryClient(
      <AuthGate>
        <div>private content</div>
      </AuthGate>
    );

    fireEvent.click(await screen.findByRole("button", { name: "开发登录" }));

    await waitFor(() => expect(screen.getByText("private content")).toBeInTheDocument());
    expect(localStorage.getItem("bws:me")).toContain("TomyJan");
  });

  test("shows configured OAuth login providers", async () => {
    vi.spyOn(window, "fetch").mockImplementation((input) => {
      const url = String(input);
      if (url.endsWith("/api/v1/me")) {
        return Promise.resolve(new Response("", { status: 401 }));
      }
      if (url.endsWith("/api/v1/oauth/providers")) {
        return Promise.resolve(
          Response.json({
            ok: true,
            data: { providers: [{ id: "qq", name: "QQ 登录", type: "qq" }], devAuth: true }
          })
        );
      }
      return Promise.reject(new Error(`unexpected request ${url}`));
    });

    renderWithQueryClient(
      <AuthGate>
        <div>private content</div>
      </AuthGate>
    );

    const qqLogin = await screen.findByRole("link", { name: "QQ 登录" });
    expect(qqLogin).toHaveAttribute("href", "/api/v1/auth/oauth/qq/login");
    expect(screen.getByRole("button", { name: "开发登录" })).toBeInTheDocument();
  });

  test("hides dev login when server disables development auth", async () => {
    vi.spyOn(window, "fetch").mockImplementation((input) => {
      const url = String(input);
      if (url.endsWith("/api/v1/me")) {
        return Promise.resolve(new Response("", { status: 401 }));
      }
      if (url.endsWith("/api/v1/oauth/providers")) {
        return Promise.resolve(
          Response.json({
            ok: true,
            data: { providers: [{ id: "qq", name: "QQ 登录", type: "qq" }], devAuth: false }
          })
        );
      }
      return Promise.reject(new Error(`unexpected request ${url}`));
    });

    renderWithQueryClient(
      <AuthGate>
        <div>private content</div>
      </AuthGate>
    );

    expect(await screen.findByRole("link", { name: "QQ 登录" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "开发登录" })).not.toBeInTheDocument();
  });

  test("does not redirect to the removed dedicated OIDC route when dev login fails", async () => {
    vi.spyOn(window, "fetch").mockImplementation((input) => {
      const url = String(input);
      if (url.endsWith("/api/v1/me")) {
        return Promise.resolve(new Response("", { status: 401 }));
      }
      if (url.endsWith("/api/v1/oauth/providers")) {
        return Promise.resolve(
          Response.json({
            ok: true,
            data: { providers: [{ id: "qq", name: "QQ 登录", type: "qq" }], devAuth: true }
          })
        );
      }
      if (url.endsWith("/api/v1/dev/login?name=TomyJan")) {
        return Promise.resolve(Response.json({ ok: false, error: { code: "not_found", message: "not found" } }));
      }
      return Promise.reject(new Error(`unexpected request ${url}`));
    });

    renderWithQueryClient(
      <AuthGate>
        <div>private content</div>
      </AuthGate>
    );

    fireEvent.click(await screen.findByRole("button", { name: "开发登录" }));

    expect(await screen.findByRole("link", { name: "QQ 登录" })).toHaveAttribute("href", "/api/v1/auth/oauth/qq/login");
  });

  test("shows friendly OAuth callback error message", async () => {
    window.history.pushState({}, "", "/?auth_error=oauth_account_already_linked");
    vi.spyOn(window, "fetch").mockImplementation((input) => {
      const url = String(input);
      if (url.endsWith("/api/v1/me")) {
        return Promise.resolve(new Response("", { status: 401 }));
      }
      if (url.endsWith("/api/v1/oauth/providers")) {
        return Promise.resolve(
          Response.json({
            ok: true,
            data: { providers: [{ id: "qq", name: "QQ 登录", type: "qq" }], devAuth: false }
          })
        );
      }
      return Promise.reject(new Error(`unexpected request ${url}`));
    });

    renderWithQueryClient(
      <AuthGate>
        <div>private content</div>
      </AuthGate>
    );

    expect(await screen.findByText("该第三方账号已绑定到其他用户。请切换账号后重试。")).toBeInTheDocument();
    expect(screen.queryByText("OAuth account binding failed")).not.toBeInTheDocument();
  });
});
