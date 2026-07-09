import { Alert, Avatar, Box, Button, Card, CardContent, Chip, Stack, Typography } from "@mui/material";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../../api/client";
import { qrImageURL } from "../../api/qr";
import type { MeResponse } from "../../api/types";
import { CloudUploadIcon, DeleteIcon } from "../../icons";
import { UserLayout } from "../../layouts/UserLayout";

export function ProfilePage() {
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
  const hasQR = Boolean(user?.qrImageUrl);
  const qrSrc = qrImageURL(user);

  return (
    <UserLayout maxWidth="lg">
      <Stack spacing={3}>
        <Box>
          <Typography variant="h4" sx={{ fontWeight: 900 }}>
            个人中心
          </Typography>
          <Typography color="text.secondary" sx={{ mt: 0.5 }}>
            维护你的账户信息和互助打卡二维码。
          </Typography>
        </Box>

        {error && <Alert severity="error">{error}</Alert>}

        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "320px minmax(0, 1fr)" },
            gap: 2
          }}
        >
          <Card variant="outlined" sx={{ alignSelf: "start" }}>
            <CardContent>
              <Stack spacing={2.5}>
                <Stack direction="row" sx={{ alignItems: "center", gap: 1.5 }}>
                  <Avatar src={user?.avatarUrl} sx={{ width: 64, height: 64, fontSize: 28 }}>
                    {user?.displayName?.[0]}
                  </Avatar>
                  <Box sx={{ minWidth: 0 }}>
                    <Typography variant="h6" sx={{ fontWeight: 850 }} noWrap>
                      {user?.displayName ?? "我"}
                    </Typography>
                    <Typography color="text.secondary" sx={{ fontSize: 13 }} noWrap>
                      {user ? "当前登录账户" : "正在加载账户信息"}
                    </Typography>
                  </Box>
                </Stack>
                <Box sx={{ display: "grid", gap: 1 }}>
                  <Stack direction="row" sx={{ alignItems: "center", justifyContent: "space-between", gap: 1 }}>
                    <Typography color="text.secondary">二维码状态</Typography>
                    <Chip size="small" color={hasQR ? "success" : "warning"} label={hasQR ? "已上传" : "未上传"} />
                  </Stack>
                  <Stack direction="row" sx={{ alignItems: "center", justifyContent: "space-between", gap: 1 }}>
                    <Typography color="text.secondary">可被互助打卡</Typography>
                    <Chip size="small" color={hasQR ? "primary" : "default"} label={hasQR ? "可用" : "不可用"} />
                  </Stack>
                </Box>
              </Stack>
            </CardContent>
          </Card>

          <Card variant="outlined">
            <CardContent>
              <Stack spacing={2.5}>
                <Stack direction={{ xs: "column", sm: "row" }} sx={{ alignItems: { sm: "center" }, justifyContent: "space-between", gap: 1.5 }}>
                  <Box>
                    <Typography variant="h6" sx={{ fontWeight: 850 }}>
                      我的二维码
                    </Typography>
                    <Typography color="text.secondary" sx={{ fontSize: 14 }}>
                      互助组成员会通过这个二维码帮你完成点位打卡。
                    </Typography>
                  </Box>
                  <Stack direction="row" sx={{ gap: 1 }}>
                    <Button component="label" variant="contained" startIcon={<CloudUploadIcon />} disabled={uploadQR.isPending}>
                      {hasQR ? "更新二维码" : "上传二维码"}
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
                    <Button color="error" variant="outlined" startIcon={<DeleteIcon />} disabled={!hasQR || deleteQR.isPending} onClick={() => deleteQR.mutate()}>
                      删除
                    </Button>
                  </Stack>
                </Stack>

                <Box
                  sx={{
                    display: "grid",
                    placeItems: "center",
                    minHeight: { xs: 360, md: 520 },
                    border: 1,
                    borderColor: "divider",
                    borderRadius: 3,
                    bgcolor: "background.paper",
                    overflow: "hidden"
                  }}
                >
                  {hasQR ? (
                    <Box component="img" src={qrSrc} alt="我的二维码" sx={{ width: "100%", height: "100%", maxHeight: 560, objectFit: "contain" }} />
                  ) : (
                    <Stack sx={{ alignItems: "center", gap: 1.5, color: "text.secondary", textAlign: "center", px: 2 }}>
                      <Typography variant="h6" sx={{ fontWeight: 800 }}>
                        尚未上传二维码
                      </Typography>
                      <Typography>上传后，互助组详情页会自动显示你的二维码。</Typography>
                    </Stack>
                  )}
                </Box>
              </Stack>
            </CardContent>
          </Card>
        </Box>
      </Stack>
    </UserLayout>
  );
}
