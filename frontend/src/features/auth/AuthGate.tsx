import { Alert, Button, CircularProgress, Stack, Typography } from "@mui/material";
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
  const authErrorMessage = oauthErrorMessage(new URLSearchParams(window.location.search).get("auth_error"));

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
        {authErrorMessage ? (
          <Alert severity="error" sx={{ width: "min(100% - 32px, 420px)" }}>
            {authErrorMessage}
          </Alert>
        ) : null}
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

function oauthErrorMessage(code: string | null): string {
  switch (code) {
    case "oauth_account_already_linked":
      return "该第三方账号已绑定到其他用户。请切换账号后重试。";
    case "oauth_state_invalid":
      return "登录状态已失效，请重新发起登录。";
    case "oauth_code_missing":
      return "登录渠道没有返回授权码，请重新发起登录。";
    case "oauth_profile_failed":
      return "登录渠道返回的信息不完整，请稍后重试。";
    case "oauth_login_required":
      return "登录状态已失效，请重新登录后再绑定账号。";
    case "oauth_binding_failed":
      return "账号绑定失败，请稍后重试。";
    default:
      return "";
  }
}
