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

function loginStatusLabel(status?: BilibiliLoginPollResponse["status"]) {
  switch (status) {
    case "pending_scan":
      return "等待扫码";
    case "pending_confirm":
      return "等待确认";
    case "expired":
      return "二维码已过期";
    case "confirmed":
      return "登录成功";
    case "failed":
      return "登录失败";
    default:
      return "等待扫码";
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
  const hasUploadedQR = Boolean(user?.qrImageUrl);
  const hasCurrentQR = currentSource === "bilibili_generated" ? isBound : hasUploadedQR;
  const qrSrc = hasCurrentQR ? qrImageURL(user) : "";
  const loginQRImage = loginQR?.imageDataUrl ?? "";
  const loginQRUnavailable = Boolean(loginQR && !loginQRImage);
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
        <Stack direction={{ xs: "column", sm: "row" }} sx={{ alignItems: { sm: "flex-end" }, justifyContent: "space-between", gap: 1.5 }}>
          <Box>
            <Typography variant="h4" sx={{ fontWeight: 900 }}>
              个人中心
            </Typography>
            <Typography color="text.secondary" sx={{ mt: 0.5, maxWidth: 620 }}>
              管理互助打卡使用的二维码来源。
            </Typography>
          </Box>
          <Stack direction="row" sx={{ gap: 1, flexWrap: "wrap" }}>
            <Chip color={isBound ? "success" : "default"} label={isBound ? "B 站已登录" : "B 站未登录"} />
            <Chip color={hasCurrentQR ? "primary" : "warning"} label={hasCurrentQR ? "二维码可用" : "缺少二维码"} />
          </Stack>
        </Stack>

        {error && <Alert severity="error">{error}</Alert>}

        <Paper
          variant="outlined"
          sx={{
            overflow: "hidden",
            borderRadius: 4,
            bgcolor: "background.paper"
          }}
        >
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", lg: "390px minmax(0, 1fr)" },
              minHeight: { lg: 680 }
            }}
          >
            <Stack
              spacing={3}
              sx={{
                p: { xs: 2, sm: 3 },
                borderRight: { lg: 1 },
                borderBottom: { xs: 1, lg: 0 },
                borderColor: "divider",
                bgcolor: "action.hover"
              }}
            >
              <Stack spacing={1.5}>
                <Stack direction="row" sx={{ alignItems: "center", justifyContent: "space-between", gap: 1.5 }}>
                  <Box>
                    <Typography variant="h6" component="h2" sx={{ fontWeight: 900 }}>
                      B 站扫码登录
                    </Typography>
                    <Typography color="text.secondary" sx={{ fontSize: 14 }}>
                      登录后可由系统生成 BWS 二维码。
                    </Typography>
                  </Box>
                  <Chip size="small" color={isBound ? "success" : "default"} label={isBound ? "已登录" : "未登录"} />
                </Stack>

                <Box
                  sx={{
                    display: "grid",
                    gridTemplateColumns: "56px minmax(0, 1fr)",
                    gap: 1.5,
                    alignItems: "center",
                    border: 1,
                    borderColor: "divider",
                    borderRadius: 3,
                    p: 1.5,
                    bgcolor: "background.paper"
                  }}
                >
                  <Avatar src={account?.faceUrl} sx={{ width: 56, height: 56 }}>
                    {(account?.uname ?? user?.displayName)?.[0]}
                  </Avatar>
                  <Box sx={{ minWidth: 0 }}>
                    <Typography sx={{ fontWeight: 850 }} noWrap>
                      {isBound ? account?.uname : "等待 B 站登录"}
                    </Typography>
                    <Typography color="text.secondary" sx={{ fontSize: 13 }} noWrap>
                      {isBound ? "账号已可用于生成二维码" : "使用 B 站客户端扫码确认"}
                    </Typography>
                  </Box>
                </Box>
              </Stack>

              <Box
                sx={{
                  display: "grid",
                  placeItems: "center",
                  minHeight: 280,
                  border: 1,
                  borderColor: loginQRUnavailable ? "warning.main" : "divider",
                  borderRadius: 4,
                  bgcolor: "background.paper",
                  p: 2
                }}
              >
                {loginQRImage ? (
                  <Stack spacing={1.5} sx={{ alignItems: "center", width: "100%" }}>
                    <Box
                      component="img"
                      src={loginQRImage}
                      alt="B 站登录二维码"
                      sx={{
                        width: 220,
                        height: 220,
                        maxWidth: "100%",
                        borderRadius: 3,
                        objectFit: "contain",
                        bgcolor: "#fff",
                        p: 1
                      }}
                    />
                    <Typography sx={{ fontWeight: 850 }}>{loginStatusLabel(loginStatus)}</Typography>
                    {isPollingLogin && <LinearProgress sx={{ width: "100%", borderRadius: 999 }} />}
                  </Stack>
                ) : (
                  <Stack spacing={1} sx={{ alignItems: "center", textAlign: "center", color: "text.secondary" }}>
                    <Typography sx={{ fontWeight: 850, color: "text.primary" }}>
                      {loginQRUnavailable ? "登录二维码暂不可用" : "尚未生成登录二维码"}
                    </Typography>
                    <Typography sx={{ fontSize: 14 }}>
                      {loginQRUnavailable ? "请重新生成二维码。" : "生成后会自动轮询扫码状态。"}
                    </Typography>
                  </Stack>
                )}
              </Box>

              <Stack spacing={1.25}>
                <Button
                  variant="contained"
                  startIcon={<SyncIcon />}
                  disabled={createLoginQR.isPending}
                  onClick={() => createLoginQR.mutate()}
                >
                  {loginQR || isBound ? "重新生成登录二维码" : "生成登录二维码"}
                </Button>
                {isBound && (
                  <Button color="error" variant="text" disabled={unbindAccount.isPending} onClick={() => unbindAccount.mutate()}>
                    退出 B 站登录
                  </Button>
                )}
              </Stack>

              <Divider />

              <Stack spacing={1.5}>
                <Box>
                  <Typography sx={{ fontWeight: 850 }}>二维码来源</Typography>
                  <Typography color="text.secondary" sx={{ fontSize: 14 }}>
                    当前使用：{sourceLabel[currentSource]}
                  </Typography>
                </Box>
                <Box
                  sx={{
                    display: "grid",
                    gridTemplateColumns: "1fr 1fr",
                    gap: 1,
                    p: 0.5,
                    border: 1,
                    borderColor: "divider",
                    borderRadius: 999,
                    bgcolor: "background.paper"
                  }}
                >
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
              </Stack>
            </Stack>

            <Stack spacing={2.5} sx={{ p: { xs: 2, sm: 3 } }}>
              <Stack direction={{ xs: "column", sm: "row" }} sx={{ alignItems: { sm: "center" }, justifyContent: "space-between", gap: 1.5 }}>
                <Box>
                  <Typography variant="h6" sx={{ fontWeight: 900 }}>
                    当前互助二维码
                  </Typography>
                  <Typography color="text.secondary" sx={{ fontSize: 14 }}>
                    {sourceLabel[currentSource]}
                  </Typography>
                </Box>
                <Chip size="small" color={hasCurrentQR ? "success" : "warning"} label={hasCurrentQR ? "可用于互助组" : "不可用"} />
              </Stack>

              <Box
                sx={{
                  display: "grid",
                  placeItems: "center",
                  minHeight: { xs: 360, md: 520 },
                  border: 1,
                  borderColor: "divider",
                  borderRadius: 4,
                  bgcolor: "action.hover",
                  overflow: "hidden",
                  p: { xs: 1.5, sm: 2 }
                }}
              >
                {qrSrc ? (
                  <Box
                    component="img"
                    src={qrSrc}
                    alt="我的二维码"
                    sx={{
                      width: "100%",
                      height: "100%",
                      maxHeight: 560,
                      objectFit: "contain",
                      borderRadius: 3,
                      bgcolor: "background.paper"
                    }}
                  />
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

              <Stack direction={{ xs: "column", sm: "row" }} sx={{ gap: 1, justifyContent: "flex-end" }}>
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
            </Stack>
          </Box>
        </Paper>
      </Stack>
    </UserLayout>
  );
}
