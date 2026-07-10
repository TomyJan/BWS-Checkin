import { QueryClient } from "@tanstack/react-query";
import { describe, expect, test, vi } from "vitest";
import { completeLogout } from "./UserLayout";

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
