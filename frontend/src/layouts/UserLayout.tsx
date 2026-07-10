import {
  AppBar,
  Avatar,
  Box,
  Button,
  Container,
  Menu,
  MenuItem,
  Stack,
  Typography
} from "@mui/material";
import { type QueryClient, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type ReactNode } from "react";
import { Link as RouterLink, useLocation } from "react-router-dom";
import { api } from "../api/client";
import type { MeResponse } from "../api/types";

interface UserLayoutProps {
  children: ReactNode;
  maxWidth?: "sm" | "md" | "lg";
}

export function UserLayout({ children, maxWidth = "md" }: UserLayoutProps) {
  const location = useLocation();
  const queryClient = useQueryClient();
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<MeResponse>("/me") });
  const user = me.data?.user;

  async function logout() {
    setAnchor(null);
    await api("/logout", { method: "POST" });
    completeLogout(queryClient);
  }

  return (
    <Box sx={{ minHeight: "100vh", bgcolor: "background.default" }}>
      <AppBar position="sticky" color="inherit" elevation={0} sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Container maxWidth="lg">
          <Stack
            direction="row"
            sx={{
              minHeight: 64,
              alignItems: "center",
              justifyContent: "space-between",
              gap: { xs: 1, sm: 2 }
            }}
          >
            <Stack direction="row" sx={{ alignItems: "center", gap: { xs: 1, sm: 2 }, minWidth: 0 }}>
              <Typography variant="h6" sx={{ fontWeight: 900, whiteSpace: "nowrap" }}>
                BWS 互助
              </Typography>
              <Stack direction="row" component="nav" aria-label="主导航" sx={{ gap: 0.5 }}>
                <Button
                  component={RouterLink}
                  to="/"
                  color={location.pathname === "/" ? "primary" : "inherit"}
                  variant={location.pathname === "/" ? "contained" : "text"}
                  sx={{ borderRadius: 999, px: { xs: 1.25, sm: 2 } }}
                >
                  互助组
                </Button>
                <Button
                  component={RouterLink}
                  to="/profile"
                  color={location.pathname === "/profile" ? "primary" : "inherit"}
                  variant={location.pathname === "/profile" ? "contained" : "text"}
                  sx={{ borderRadius: 999, px: { xs: 1.25, sm: 2 } }}
                >
                  个人中心
                </Button>
              </Stack>
            </Stack>
            <Button
              color="inherit"
              onClick={(event) => setAnchor(event.currentTarget)}
              startIcon={
                <Avatar src={user?.avatarUrl} sx={{ width: 30, height: 30 }}>
                  {user?.displayName?.[0]}
                </Avatar>
              }
              sx={{ borderRadius: 999, minWidth: 0, textTransform: "none" }}
            >
              <Box component="span" sx={{ display: { xs: "none", sm: "inline" } }}>
                {user?.displayName ?? "我"}
              </Box>
            </Button>
          </Stack>
        </Container>
      </AppBar>

      <Menu anchorEl={anchor} open={Boolean(anchor)} onClose={() => setAnchor(null)} anchorOrigin={{ vertical: "bottom", horizontal: "right" }}>
        <MenuItem disabled>
          {user?.displayName ?? "我"}
        </MenuItem>
        <MenuItem onClick={() => void logout()}>退出登录</MenuItem>
      </Menu>

      <Box component="main" sx={{ py: { xs: 2, md: 4 } }}>
        <Container maxWidth={maxWidth}>{children}</Container>
      </Box>
    </Box>
  );
}

export function completeLogout(queryClient: QueryClient, location: Pick<Location, "assign"> = window.location) {
  localStorage.removeItem("bws:me");
  queryClient.clear();
  location.assign("/");
}
