import { Alert, Avatar, Box, Button, Card, CardContent, Chip, Divider, LinearProgress, Stack, Typography } from "@mui/material";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import {
  api,
  createBilibiliLoginQRCode,
  getBilibiliAccount,
  getMe,
  pollBilibiliLoginQRCode,
  setQRSource,
  unbindBilibiliAccount
} from "../../api/client";
import { qrImageURL } from "../../api/qr";
import type { BilibiliLoginQRCodeResponse, BilibiliLoginPollResponse, MeResponse } from "../../api/types";
import { CloudUploadIcon, DeleteIcon } from "../../icons";
import { UserLayout } from "../../layouts/UserLayout";

const sourceLabel = {
  uploaded: "手动上传",
  bilibili_generated: "B 站账号生成"
} as const;

function loginStatusLabel(status?: BilibiliLoginPollResponse["status"]) {
  switch (status) {
    case "pending_scan":
      return "等待扫码";
    case "pending_confirm":
      return "等待确认";
    case "expired":
      return "二维码已过期";
    case "confirmed":
      return "绑定成功";
    case "failed":
      return "绑定失败";
    default:
      return "";
  }
}

export function ProfilePage() {
  const queryClient = useQueryClient();
  const [error, setError] = useState("");
  const [loginQR, setLoginQR] = useState<BilibiliLoginQRCodeResponse["qrcode"] | null>(null);
  const [loginStatus, setLoginStatus] = useState<BilibiliLoginPollResponse["status"]>();
  const me = useQuery({ queryKey: ["me"], queryFn: getMe });
  const bilibiliAccount = useQuery({ queryKey: ["bilibili-account"], queryFn: getBilibiliAccount });

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

  const createLoginQR = useMutation({
    mutationFn: createBilibiliLoginQRCode,
    onError: (err) => setError(err instanceof Error ? err.message : "创建登录二维码失败"),
    onSuccess: (data) => {
      setError("");
      setLoginStatus(undefined);
      setLoginQR(data.qrcode);
    }
  });

  const pollLogin = useMutation({
    mutationFn: () => {
      if (!loginQR?.qrcodeKey) throw new Error("登录二维码不存在");
      return pollBilibiliLoginQRCode(loginQR.qrcodeKey);
    },
    onError: (err) => setError(err instanceof Error ? err.message : "检查登录状态失败"),
    onSuccess: async (data) => {
      setError("");
      setLoginStatus(data.status);
      if (data.status === "confirmed" && data.account) {
        queryClient.setQueryData(["bilibili-account"], { bound: true, account: data.account });
        await queryClient.invalidateQueries({ queryKey: ["me"] });
      }
    }
  });

  const changeQRSource = useMutation({
    mutationFn: setQRSource,
    onError: (err) => setError(err instanceof Error ? err.message : "切换二维码来源失败"),
    onSuccess: async (data) => {
      setError("");
      queryClient.setQueryData(["me"], data);
      await queryClient.invalidateQueries({ queryKey: ["me"] });
    }
  });

  const unbindAccount = useMutation({
    mutationFn: unbindBilibiliAccount,
    onError: (err) => setError(err instanceof Error ? err.message : "解绑 B 站账号失败"),
    onSuccess: async () => {
      setError("");
      setLoginQR(null);
      setLoginStatus(undefined);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["bilibili-account"] }),
        queryClient.invalidateQueries({ queryKey: ["me"] })
      ]);
    }
  });

  const user = me.data?.user;
  const account = bilibiliAccount.data?.account;
  const isBound = Boolean(bilibiliAccount.data?.bound && account);
  const currentSource = user?.qrSource ?? "uploaded";
  const hasQR = Boolean(user?.qrImageUrl || (currentSource === "bilibili_generated" && isBound));
  const qrSrc = qrImageURL(user);
  const loginQRImage = loginQR?.imageDataUrl ?? loginQR?.url ?? "";
  const isPollingLogin = Boolean(loginQR && loginStatus !== "confirmed" && loginStatus !== "expired" && loginStatus !== "failed");

  useEffect(() => {
    if (!loginQR?.qrcodeKey || !isPollingLogin || pollLogin.isPending) return;
    const delay = loginStatus ? 1800 : 0;
    const timer = window.setTimeout(() => pollLogin.mutate(), delay);
    return () => window.clearTimeout(timer);
  }, [isPollingLogin, loginQR?.qrcodeKey, loginStatus, pollLogin]);

  return (
    <UserLayout maxWidth="lg">
      <Stack spacing={3}>
        <Stack direction={{ xs: "column", sm: "row" }} sx={{ alignItems: { sm: "center" }, justifyContent: "space-between", gap: 1.5 }}>
          <Box>
            <Typography variant="h4" sx={{ fontWeight: 900 }}>
              个人中心
            </Typography>
            <Typography color="text.secondary" sx={{ mt: 0.5 }}>
              绑定 B 站账号或上传 BWS 二维码，互助组会使用这里的二维码。
            </Typography>
          </Box>
          <Stack direction="row" sx={{ gap: 1, flexWrap: "wrap" }}>
            <Chip color={isBound ? "success" : "default"} label={isBound ? "B 站已绑定" : "B 站未绑定"} />
            <Chip color={hasQR ? "primary" : "warning"} label={hasQR ? "二维码可用" : "缺少二维码"} />
          </Stack>
        </Stack>

        {error && <Alert severity="error">{error}</Alert>}

        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", lg: "420px minmax(0, 1fr)" },
            gap: 2.5,
            alignItems: "start"
          }}
        >
          <Stack spacing={2}>
          <Card variant="outlined">
            <CardContent>
              <Stack spacing={2.5}>
                <Stack direction="row" sx={{ alignItems: "center", justifyContent: "space-between", gap: 1.5 }}>
                  <Box>
                    <Typography variant="h6" component="h2" sx={{ fontWeight: 900 }}>
                      B 站扫码登录
                    </Typography>
                    <Typography color="text.secondary" sx={{ fontSize: 14, mt: 0.5 }}>
                      绑定后可由系统生成你的 BWS 二维码。
                    </Typography>
                  </Box>
                  <Chip size="small" color={isBound ? "success" : "default"} label={isBound ? "已绑定" : "未绑定"} />
                </Stack>

                <Stack direction="row" sx={{ alignItems: "center", gap: 1.5, minWidth: 0 }}>
                  <Avatar src={account?.faceUrl} sx={{ width: 52, height: 52 }}>
                    {(account?.uname ?? user?.displayName)?.[0]}
                  </Avatar>
                  <Box sx={{ minWidth: 0 }}>
                    <Typography sx={{ fontWeight: 850 }} noWrap>
                      {isBound ? account?.uname : "尚未绑定 B 站账号"}
                    </Typography>
                    <Typography color="text.secondary" sx={{ fontSize: 13 }} noWrap>
                      {isBound ? `UID ${account?.mid}` : "使用 B 站客户端扫码确认登录"}
                    </Typography>
                  </Box>
                </Stack>

                {loginQR && (
                  <Box
                    sx={{
                      display: "grid",
                      gridTemplateColumns: "128px minmax(0, 1fr)",
                      gap: 2,
                      alignItems: "center",
                      border: 1,
                      borderColor: "divider",
                      borderRadius: 3,
                      p: 1.5,
                      bgcolor: "action.hover"
                    }}
                  >
                    <Box
                      component="img"
                      src={loginQRImage}
                      alt="B 站登录二维码"
                      sx={{
                        width: 128,
                        height: 128,
                        borderRadius: 2,
                        bgcolor: "background.paper",
                        objectFit: "contain"
                      }}
                    />
                    <Stack spacing={1} sx={{ minWidth: 0 }}>
                      <Typography sx={{ fontWeight: 850 }}>
                        {loginStatus ? loginStatusLabel(loginStatus) : "等待扫码"}
                      </Typography>
                      <Typography color="text.secondary" sx={{ fontSize: 14 }}>
                        {loginStatus === "confirmed" ? "B 站账号已绑定。" : "保持此页面打开，系统会自动刷新扫码状态。"}
                      </Typography>
                      {isPollingLogin && <LinearProgress sx={{ borderRadius: 999 }} />}
                    </Stack>
                  </Box>
                )}

                <Stack direction="row" sx={{ gap: 1, flexWrap: "wrap" }}>
                  <Button variant="contained" disabled={createLoginQR.isPending} onClick={() => createLoginQR.mutate()}>
                    {loginQR || isBound ? "重新生成登录二维码" : "生成登录二维码"}
                  </Button>
                  {isBound && (
                    <Button color="error" variant="text" disabled={unbindAccount.isPending} onClick={() => unbindAccount.mutate()}>
                      解绑 B 站账号
                    </Button>
                  )}
                </Stack>
              </Stack>
            </CardContent>
          </Card>

          <Card variant="outlined">
            <CardContent>
              <Stack spacing={2.25}>
                <Box>
                  <Typography variant="h6" component="h2" sx={{ fontWeight: 900 }}>
                    二维码来源
                  </Typography>
                  <Typography color="text.secondary" sx={{ fontSize: 14, mt: 0.5 }}>
                    当前使用：{sourceLabel[currentSource]}
                  </Typography>
                </Box>
                <Stack spacing={1}>
                  <Button
                    fullWidth
                    variant={currentSource === "bilibili_generated" ? "contained" : "outlined"}
                    disabled={!isBound || changeQRSource.isPending}
                    onClick={() => changeQRSource.mutate("bilibili_generated")}
                  >
                    使用 B 站二维码
                  </Button>
                  <Button
                    fullWidth
                    variant={currentSource === "uploaded" ? "contained" : "outlined"}
                    disabled={changeQRSource.isPending}
                    onClick={() => changeQRSource.mutate("uploaded")}
                  >
                    使用上传二维码
                  </Button>
                </Stack>
                <Divider />
                <Stack spacing={1}>
                  <Button component="label" variant="outlined" startIcon={<CloudUploadIcon />} disabled={uploadQR.isPending}>
                    {user?.qrImageUrl ? "更新上传二维码" : "上传 BWS 二维码"}
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
                  <Button color="error" variant="text" startIcon={<DeleteIcon />} disabled={!user?.qrImageUrl || deleteQR.isPending} onClick={() => deleteQR.mutate()}>
                    删除上传图
                  </Button>
                </Stack>
              </Stack>
            </CardContent>
          </Card>
          </Stack>

          <Card variant="outlined">
            <CardContent>
              <Stack spacing={2.5}>
                <Stack direction={{ xs: "column", sm: "row" }} sx={{ alignItems: { sm: "center" }, justifyContent: "space-between", gap: 1.5 }}>
                  <Box>
                    <Typography variant="h6" sx={{ fontWeight: 850 }}>
                      我的二维码
                    </Typography>
                    <Typography color="text.secondary" sx={{ fontSize: 14 }}>
                      {sourceLabel[currentSource]}
                    </Typography>
                  </Box>
                  <Chip size="small" color={hasQR ? "success" : "warning"} label={hasQR ? "可用" : "不可用"} />
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
                  {hasQR && qrSrc ? (
                    <Box component="img" src={qrSrc} alt="我的二维码" sx={{ width: "100%", height: "100%", maxHeight: 560, objectFit: "contain" }} />
                  ) : (
                    <Stack sx={{ alignItems: "center", gap: 1.5, color: "text.secondary", textAlign: "center", px: 2 }}>
                      <Typography variant="h6" sx={{ fontWeight: 800 }}>
                        暂无可用二维码
                      </Typography>
                      <Typography>绑定 B 站账号或上传图片后可用。</Typography>
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
