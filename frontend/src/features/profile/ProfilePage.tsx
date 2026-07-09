import { Alert, Avatar, Box, Button, Card, CardContent, Container, Stack, Typography } from "@mui/material";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../../api/client";
import type { MeResponse } from "../../api/types";
import { ArrowBackIcon, CloudUploadIcon, DeleteIcon } from "../../icons";

export function ProfilePage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [error, setError] = useState("");
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<MeResponse>("/me") });

  const uploadQR = useMutation({
    mutationFn: async (file: File) => {
      const form = new FormData();
      form.append("file", file);
      await api("/me/qr/upload", { method: "POST", body: form });
    },
    onError: (err) => setError(err instanceof Error ? err.message : "上传二维码失败"),
    onSuccess: async () => {
      setError("");
      await queryClient.invalidateQueries({ queryKey: ["me"] });
    }
  });

  const deleteQR = useMutation({
    mutationFn: () => api("/me/qr/delete", { method: "POST" }),
    onError: (err) => setError(err instanceof Error ? err.message : "删除二维码失败"),
    onSuccess: async () => {
      setError("");
      await queryClient.invalidateQueries({ queryKey: ["me"] });
    }
  });

  const user = me.data?.user;

  return (
    <Box sx={{ minHeight: "100vh", bgcolor: "background.default", py: { xs: 2, md: 4 } }}>
      <Container maxWidth="sm">
        <Stack spacing={2.5}>
          <Button startIcon={<ArrowBackIcon />} onClick={() => navigate("/")} sx={{ alignSelf: "flex-start" }}>
            返回
          </Button>
          <Stack direction="row" sx={{ alignItems: "center", gap: 1.5 }}>
            <Avatar src={user?.avatarUrl} sx={{ width: 56, height: 56 }}>
              {user?.displayName?.[0]}
            </Avatar>
            <Box>
              <Typography variant="h5" sx={{ fontWeight: 850 }}>
                {user?.displayName ?? "个人资料"}
              </Typography>
              <Typography color="text.secondary" sx={{ fontSize: 14 }}>
                管理你的二维码
              </Typography>
            </Box>
          </Stack>

          {error && <Alert severity="error">{error}</Alert>}

          <Card variant="outlined">
            <CardContent>
              <Stack spacing={2}>
                <Box
                  sx={{
                    display: "grid",
                    placeItems: "center",
                    minHeight: 320,
                    borderRadius: 4,
                    bgcolor: "action.hover",
                    overflow: "hidden"
                  }}
                >
                  {user?.qrImageUrl ? (
                    <Box
                      component="img"
                      src={user.qrImageUrl}
                      alt="我的二维码"
                      sx={{ width: "100%", maxHeight: 420, objectFit: "contain" }}
                    />
                  ) : (
                    <Typography color="text.secondary">尚未上传二维码</Typography>
                  )}
                </Box>
                <Stack direction={{ xs: "column", sm: "row" }} sx={{ gap: 1 }}>
                  <Button component="label" fullWidth variant="contained" startIcon={<CloudUploadIcon />} disabled={uploadQR.isPending}>
                    {user?.qrImageUrl ? "更新二维码" : "上传二维码"}
                    <input
                      hidden
                      accept="image/png,image/jpeg,image/webp"
                      type="file"
                      onChange={(event) => {
                        const file = event.target.files?.[0];
                        event.currentTarget.value = "";
                        if (file) uploadQR.mutate(file);
                      }}
                    />
                  </Button>
                  <Button
                    fullWidth
                    color="error"
                    variant="outlined"
                    startIcon={<DeleteIcon />}
                    disabled={!user?.qrImageUrl || deleteQR.isPending}
                    onClick={() => deleteQR.mutate()}
                  >
                    删除二维码
                  </Button>
                </Stack>
              </Stack>
            </CardContent>
          </Card>
        </Stack>
      </Container>
    </Box>
  );
}
