import "@testing-library/jest-dom/vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
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
    setOnline(true);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  test("does not use cached user when the online session is rejected", async () => {
    localStorage.setItem(
      "bws:me",
      JSON.stringify({
        user: { id: "cached-user", displayName: "Cached", avatarUrl: "", qrImageUrl: "" }
      })
    );
    vi.spyOn(window, "fetch").mockResolvedValue(new Response("", { status: 401 }));

    renderWithQueryClient(
      <AuthGate>
        <div>private content</div>
      </AuthGate>
    );

    await waitFor(() => expect(screen.getByRole("button", { name: "登录" })).toBeInTheDocument());
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
});
