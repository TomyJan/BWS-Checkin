import { Button, CircularProgress, Stack, Typography } from "@mui/material";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { PropsWithChildren } from "react";
import { api, getOAuthProviders } from "../../api/client";
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
  const oauthProviders = useQuery({
    queryKey: ["oauth-providers"],
    queryFn: getOAuthProviders,
    enabled: me.isError,
    retry: false
  });
  const devAuthEnabled = oauthProviders.data?.devAuth ?? false;

  async function login() {
    try {
      const response = await api<MeResponse>("/dev/login?name=TomyJan", { method: "POST" });
      cacheMe(response);
      queryClient.setQueryData(["me"], response);
    } catch {
      return;
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
        {(oauthProviders.data?.providers ?? []).map((provider) => (
          <Button key={provider.id} variant="contained" href={`/api/v1/auth/oauth/${provider.id}/login`}>
            {provider.name}
          </Button>
        ))}
        {devAuthEnabled ? (
          <Button variant={(oauthProviders.data?.providers?.length ?? 0) > 0 ? "outlined" : "contained"} onClick={login}>
            开发登录
          </Button>
        ) : null}
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
