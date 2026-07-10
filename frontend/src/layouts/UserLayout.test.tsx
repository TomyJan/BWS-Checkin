import "@testing-library/jest-dom/vitest";
import { QueryClient } from "@tanstack/react-query";
import { QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, test, vi } from "vitest";
import { completeLogout, UserLayout } from "./UserLayout";

function renderUserLayout() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } }
  });
  queryClient.setQueryData(["me"], {
    user: { id: "u1", displayName: "TomyJan", avatarUrl: "", qrImageUrl: "", qrSource: "uploaded" }
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/"]}>
        <UserLayout>
          <div>content</div>
        </UserLayout>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("UserLayout", () => {
  test("shows a Beta badge beside the brand title", () => {
    vi.spyOn(window, "fetch").mockRejectedValue(new Error("unused"));

    renderUserLayout();

    expect(screen.getByText("BWS 互助")).toBeInTheDocument();
    expect(screen.getByText("Beta")).toBeInTheDocument();
  });
});

describe("completeLogout", () => {
  test("clears cached state and reloads to the entry page", () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(["me"], { user: { id: "u1", displayName: "TomyJan" } });
    localStorage.setItem("bws:me", JSON.stringify({ user: { id: "u1", displayName: "TomyJan" } }));
    const location = { assign: vi.fn() };

    completeLogout(queryClient, location);

    expect(queryClient.getQueryData(["me"])).toBeUndefined();
    expect(localStorage.getItem("bws:me")).toBeNull();
    expect(location.assign).toHaveBeenCalledWith("/");
  });
});
