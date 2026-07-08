import { Button, CircularProgress, Stack, Typography } from "@mui/material";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { PropsWithChildren } from "react";
import { api } from "../../api/client";
import type { MeResponse } from "../../api/types";

export function AuthGate({ children }: PropsWithChildren) {
  const queryClient = useQueryClient();
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<MeResponse>("/me"), retry: false });

  async function login() {
    await fetch("/api/v1/dev/login?name=TomyJan", { method: "POST", credentials: "include" });
    await queryClient.invalidateQueries({ queryKey: ["me"] });
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
