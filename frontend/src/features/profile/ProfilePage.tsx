import {
  Alert,
  Avatar,
  Box,
  Button,
  Chip,
  Divider,
  LinearProgress,
  Paper,
  Stack,
  Typography
} from "@mui/material";
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
import type { BilibiliLoginPollResponse, BilibiliLoginQRCodeResponse } from "../../api/types";
import { CloudUploadIcon, DeleteIcon, SyncIcon } from "../../icons";
import { UserLayout } from "../../layouts/UserLayout";

const sourceLabel = {
  uploaded: "上传图片",
  bilibili_generated: "B 站生成"
} as const;

function loginStatusText(status?: BilibiliLoginPollResponse["status"]) {
  switch (status) {
    case "pending_confirm":
      return "等待 B 站客户端确认";
    case "expired":
      return "登录二维码已过期";
    case "confirmed":
      return "B 站扫码登录完成";
    case "failed":
      return "B 站扫码登录失败";
    case "pending_scan":
    default:
      return "等待 B 站客户端扫码";
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
    onError: (err) => setError(err instanceof Error ? err.message : "退出 B 站登录失败"),
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
  const hasUploadedQR = Boolean(user?.qrImageUrl);
  const hasCurrentQR = currentSource === "bilibili_generated" ? isBound : hasUploadedQR;
  const qrSrc = hasCurrentQR ? qrImageURL(user) : "";
  const loginQRImage = loginQR?.imageDataUrl ?? "";
  const loginQRMissingImage = Boolean(loginQR && !loginQRImage);
  const isPollingLogin = Boolean(loginQR && loginStatus !== "confirmed" && loginStatus !== "expired" && loginStatus !== "failed");

  useEffect(() => {
    if (!loginQR?.qrcodeKey || !isPollingLogin || pollLogin.isPending) return;
    const timer = window.setTimeout(() => pollLogin.mutate(), 2000);
    return () => window.clearTimeout(timer);
  }, [isPollingLogin, loginQR?.qrcodeKey, loginStatus, pollLogin]);

  return (
    <UserLayout maxWidth="lg">
      <Stack spacing={3}>
        <Stack direction={{ xs: "column", sm: "row" }} sx={{ alignItems: { sm: "flex-end" }, justifyContent: "space-between", gap: 1.5 }}>
          <Box>
            <Typography variant="h4" sx={{ fontWeight: 900 }}>
              个人中心
            </Typography>
            <Typography color="text.secondary" sx={{ mt: 0.5, maxWidth: 620 }}>
              选择互助组中展示的二维码来源。
            </Typography>
          </Box>
          <Stack direction="row" sx={{ gap: 1, flexWrap: "wrap" }}>
            <Chip color={isBound ? "success" : "default"} label={isBound ? "B 站已登录" : "B 站未登录"} />
            <Chip color={hasCurrentQR ? "primary" : "warning"} label={hasCurrentQR ? `${sourceLabel[currentSource]}可用` : "缺少二维码"} />
          </Stack>
        </Stack>

        {error && <Alert severity="error">{error}</Alert>}

        <Paper
          data-testid="profile-workbench"
          variant="outlined"
          sx={{
            overflow: "hidden",
            borderRadius: 4,
            bgcolor: "background.paper",
            boxShadow: "0 24px 70px rgba(15, 23, 42, 0.08)"
          }}
        >
          <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", lg: "380px minmax(0, 1fr)" }, minHeight: { lg: 650 } }}>
            <Stack
              spacing={2.5}
              sx={{
                p: { xs: 2, sm: 3 },
                borderRight: { lg: 1 },
                borderBottom: { xs: 1, lg: 0 },
                borderColor: "divider",
                bgcolor: "background.default"
              }}
            >
              <Paper variant="outlined" sx={{ borderRadius: 3, p: 2.25, bgcolor: "background.paper" }}>
                <Stack spacing={2}>
                  <Box>
                    <Typography variant="h6" component="h2" sx={{ fontWeight: 900 }}>
                      B 站扫码登录
                    </Typography>
                    <Typography color="text.secondary" sx={{ fontSize: 14, mt: 0.5 }}>
                      扫码后可使用 B 站账号生成 BWS 二维码。
                    </Typography>
                  </Box>

                  {isBound && (
                    <Box
                      sx={{
                        display: "grid",
                        gridTemplateColumns: "48px minmax(0, 1fr)",
                        gap: 1.5,
                        alignItems: "center",
                        border: 1,
                        borderColor: "divider",
                        borderRadius: 3,
                        p: 1.5,
                        bgcolor: "action.hover"
                      }}
                    >
                      <Avatar src={account?.faceUrl} sx={{ width: 48, height: 48 }}>
                        {account?.uname?.[0] ?? user?.displayName?.[0]}
                      </Avatar>
                      <Box sx={{ minWidth: 0 }}>
                        <Typography sx={{ fontWeight: 850 }} noWrap>
                          {account?.uname}
                        </Typography>
                        <Typography color="text.secondary" sx={{ fontSize: 13 }} noWrap>
                          B 站账号已登录
                        </Typography>
                      </Box>
                    </Box>
                  )}

                  <Box
                    sx={{
                      display: "grid",
                      placeItems: "center",
                      minHeight: 252,
                      border: 1,
                      borderColor: loginQRMissingImage ? "error.main" : "divider",
                      borderRadius: 3,
                      bgcolor: "background.paper",
                      p: 2
                    }}
                  >
                    {loginQRImage ? (
                      <Box
                        component="img"
                        src={loginQRImage}
                        alt="B 站登录二维码"
                        sx={{ width: 220, height: 220, maxWidth: "100%", borderRadius: 2, objectFit: "contain", bgcolor: "#fff", p: 1 }}
                      />
                    ) : (
                      <Stack spacing={0.75} sx={{ alignItems: "center", textAlign: "center", color: "text.secondary" }}>
                        <Typography sx={{ fontWeight: 850, color: "text.primary" }}>
                          {loginQRMissingImage ? "接口未返回二维码图片" : "尚未生成登录二维码"}
                        </Typography>
                        <Typography sx={{ fontSize: 14 }}>
                          {loginQRMissingImage ? "请确认后端已重启到最新版本，或重新生成。" : "生成后会自动轮询扫码状态。"}
                        </Typography>
                      </Stack>
                    )}
                  </Box>

                  {loginQRImage && (
                    <Stack spacing={1}>
                      <Stack direction="row" sx={{ alignItems: "center", gap: 1 }}>
                        <Box sx={{ width: 8, height: 8, borderRadius: "50%", bgcolor: "primary.main", flex: "0 0 auto" }} />
                        <Typography color="text.secondary" sx={{ fontSize: 14, fontWeight: 700 }}>
                          {loginStatusText(loginStatus)}
                        </Typography>
                      </Stack>
                      {isPollingLogin && <LinearProgress sx={{ borderRadius: 999 }} />}
                    </Stack>
                  )}

                  {loginQRMissingImage && (
                    <Alert severity="error" variant="outlined" sx={{ borderRadius: 3 }}>
                      不是扫码失败，当前页面没有拿到可渲染的二维码图片数据。
                    </Alert>
                  )}

                  <Stack spacing={1}>
                    <Button variant="contained" startIcon={<SyncIcon />} disabled={createLoginQR.isPending} onClick={() => createLoginQR.mutate()}>
                      {loginQR || isBound ? "重新生成登录二维码" : "生成登录二维码"}
                    </Button>
                    {isBound && (
                      <Button color="error" variant="text" disabled={unbindAccount.isPending} onClick={() => unbindAccount.mutate()}>
                        退出 B 站登录
                      </Button>
                    )}
                  </Stack>
                </Stack>
              </Paper>

              <Box>
                <Typography color="text.secondary" sx={{ fontSize: 13, fontWeight: 900, mb: 1 }}>
                  二维码来源
                </Typography>
                <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 0.75, p: 0.5, border: 1, borderColor: "divider", borderRadius: 999, bgcolor: "background.paper" }}>
                  <Button
                    variant={currentSource === "uploaded" ? "contained" : "text"}
                    disabled={changeQRSource.isPending}
                    onClick={() => changeQRSource.mutate("uploaded")}
                    sx={{ borderRadius: 999 }}
                  >
                    上传图片
                  </Button>
                  <Button
                    variant={currentSource === "bilibili_generated" ? "contained" : "text"}
                    disabled={!isBound || changeQRSource.isPending}
                    onClick={() => changeQRSource.mutate("bilibili_generated")}
                    sx={{ borderRadius: 999 }}
                  >
                    B 站生成
                  </Button>
                </Box>
              </Box>

              <Paper variant="outlined" sx={{ borderRadius: 3, p: 2, bgcolor: "background.paper" }}>
                <Stack spacing={1.5}>
                  <Box>
                    <Typography sx={{ fontWeight: 900 }}>上传二维码</Typography>
                    <Typography color="text.secondary" sx={{ fontSize: 14, mt: 0.5 }}>
                      不登录 B 站时可继续使用上传图片。
                    </Typography>
                  </Box>
                  <Button component="label" variant="outlined" startIcon={<CloudUploadIcon />} disabled={uploadQR.isPending}>
                    {hasUploadedQR ? "更新上传二维码" : "上传 BWS 二维码"}
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
                  <Button color="error" variant="text" startIcon={<DeleteIcon />} disabled={!hasUploadedQR || deleteQR.isPending} onClick={() => deleteQR.mutate()}>
                    删除上传图
                  </Button>
                </Stack>
              </Paper>
            </Stack>

            <Stack spacing={2.25} sx={{ p: { xs: 2, sm: 3 }, minHeight: { lg: 650 } }}>
              <Stack direction={{ xs: "column", sm: "row" }} sx={{ alignItems: { sm: "center" }, justifyContent: "space-between", gap: 1.5 }}>
                <Box>
                  <Typography variant="h6" sx={{ fontWeight: 900 }}>
                    当前互助二维码
                  </Typography>
                  <Typography color="text.secondary" sx={{ fontSize: 14 }}>
                    {sourceLabel[currentSource]}
                  </Typography>
                </Box>
                <Chip size="small" color={hasCurrentQR ? "primary" : "warning"} label={hasCurrentQR ? "可用于互助组" : "不可用"} />
              </Stack>

              <Box
                sx={{
                  display: "grid",
                  placeItems: "center",
                  flex: 1,
                  minHeight: { xs: 360, md: 520 },
                  border: 1,
                  borderColor: "divider",
                  borderRadius: 4,
                  bgcolor: "background.default",
                  overflow: "hidden",
                  p: { xs: 1.5, sm: 2 }
                }}
              >
                {qrSrc ? (
                  <Box
                    data-testid="current-qr-device"
                    sx={{
                      width: "min(360px, 100%)",
                      aspectRatio: "9 / 14",
                      border: "10px solid",
                      borderColor: "text.primary",
                      borderRadius: 5,
                      bgcolor: "#fff",
                      boxShadow: "0 30px 80px rgba(15, 23, 42, 0.16)",
                      display: "grid",
                      placeItems: "center",
                      p: { xs: 2, sm: 3 }
                    }}
                  >
                    <Box
                      component="img"
                      src={qrSrc}
                      alt="我的二维码"
                      sx={{
                        width: "100%",
                        aspectRatio: "1 / 1",
                        objectFit: "contain",
                        borderRadius: 2,
                        bgcolor: "#fff"
                      }}
                    />
                  </Box>
                ) : (
                  <Stack sx={{ alignItems: "center", gap: 1, color: "text.secondary", textAlign: "center", px: 2 }}>
                    <Typography variant="h6" sx={{ fontWeight: 850, color: "text.primary" }}>
                      暂无可用二维码
                    </Typography>
                    <Typography sx={{ maxWidth: 360 }}>
                      先完成 B 站扫码登录，或上传一张 BWS 二维码图片。
                    </Typography>
                  </Stack>
                )}
              </Box>

              <Stack direction="row" sx={{ justifyContent: "flex-end", flexWrap: "wrap", gap: 1 }}>
                <Button component="label" variant="outlined" startIcon={<CloudUploadIcon />} disabled={uploadQR.isPending}>
                  替换当前图片
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
              </Stack>
            </Stack>
          </Box>
        </Paper>
      </Stack>
    </UserLayout>
  );
}
