import { Button, CircularProgress, Stack, Typography } from "@mui/material";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { PropsWithChildren } from "react";
import { api } from "../../api/client";
import type { MeResponse } from "../../api/types";

const CACHED_ME_KEY = "bws:me";

export function AuthGate({ children }: PropsWithChildren) {
  const queryClient = useQueryClient();
  const me = useQuery({
    queryKey: ["me"],
    queryFn: async () => {
      try {
        const response = await api<MeResponse>("/me");
        cacheMe(response);
        return response;
      } catch (error) {
        if (!navigator.onLine) {
          const cached = cachedMe();
          if (cached) return cached;
        }
        localStorage.removeItem(CACHED_ME_KEY);
        throw error;
      }
    },
    retry: false
  });

  async function login() {
    try {
      const response = await api<MeResponse>("/dev/login?name=TomyJan", { method: "POST" });
      cacheMe(response);
      queryClient.setQueryData(["me"], response);
    } catch {
      window.location.assign("/auth/oidc/login");
    }
  }

  if (me.isLoading) {
    return (
      <Stack sx={{ minHeight: "100vh", alignItems: "center", justifyContent: "center" }}>
        <CircularProgress />
      </Stack>
    );
  }

  if (me.isError) {
    return (
      <Stack spacing={2} sx={{ minHeight: "100vh", alignItems: "center", justifyContent: "center" }}>
        <Typography variant="h4" sx={{ fontWeight: 800 }}>
          BWS Checkin
        </Typography>
        <Button variant="contained" onClick={login}>
          登录
        </Button>
      </Stack>
    );
  }

  return children;
}

function cacheMe(me: MeResponse) {
  localStorage.setItem(CACHED_ME_KEY, JSON.stringify(me));
}

function cachedMe(): MeResponse | null {
  const raw = localStorage.getItem(CACHED_ME_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as MeResponse;
  } catch {
    return null;
  }
}
