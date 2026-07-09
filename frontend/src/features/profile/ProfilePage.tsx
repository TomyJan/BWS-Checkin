import {
  Alert,
  Avatar,
  Box,
  Button,
  Chip,
  LinearProgress,
  Paper,
  Stack,
  Typography
} from "@mui/material";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import {
  api,
  createBilibiliLoginQRCode,
  getBilibiliAccount,
  getMe,
  getOAuthAccounts,
  getOAuthProviders,
  pollBilibiliLoginQRCode,
  setQRSource,
  unbindBilibiliAccount
} from "../../api/client";
import { qrImageURL } from "../../api/qr";
import type { BilibiliLoginPollResponse, BilibiliLoginQRCodeResponse } from "../../api/types";
import { CloudUploadIcon, DeleteIcon, SyncIcon } from "../../icons";
import { UserLayout } from "../../layouts/UserLayout";
import { qrCodeDataURL } from "../../utils/qrCode";

const panelRadius = "16px";
const controlSurfaceRadius = "14px";

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

function withReloadKey(url: string, key: string) {
  if (!url) return "";
  return `${url}${url.includes("?") ? "&" : "?"}v=${encodeURIComponent(key)}`;
}

export function ProfilePage() {
  const queryClient = useQueryClient();
  const [error, setError] = useState("");
  const [loginQR, setLoginQR] = useState<BilibiliLoginQRCodeResponse["qrcode"] | null>(null);
  const [loginStatus, setLoginStatus] = useState<BilibiliLoginPollResponse["status"]>();
  const [sourceMode, setSourceMode] = useState<"uploaded" | "bilibili_generated">("uploaded");
  const [qrReloadNonce, setQRReloadNonce] = useState(0);
  const userSelectedSourceRef = useRef(false);
  const me = useQuery({ queryKey: ["me"], queryFn: getMe });
  const bilibiliAccount = useQuery({ queryKey: ["bilibili-account"], queryFn: getBilibiliAccount });
  const oauthProviders = useQuery({ queryKey: ["oauth-providers"], queryFn: getOAuthProviders });
  const oauthAccounts = useQuery({ queryKey: ["oauth-accounts"], queryFn: getOAuthAccounts });
  const user = me.data?.user;
  const account = bilibiliAccount.data?.account;
  const providers = oauthProviders.data?.providers ?? [];
  const linkedAccounts = oauthAccounts.data?.accounts ?? [];
  const isBound = Boolean(bilibiliAccount.data?.bound && account);
  const currentSource = user?.qrSource ?? "uploaded";
  const hasCurrentQR = Boolean(user?.qrImageUrl);
  const hasUploadedQR = currentSource === "uploaded" && hasCurrentQR;
  const qrSrc = hasCurrentQR ? withReloadKey(qrImageURL(user), `${currentSource}-${qrReloadNonce}`) : "";
  const loginQRImage = loginQR?.imageDataUrl || (loginQR?.url ? qrCodeDataURL(loginQR.url) : "");
  const loginQRMissingImage = Boolean(loginQR && !loginQRImage);
  const isPollingLogin = Boolean(loginQR && loginStatus !== "confirmed" && loginStatus !== "expired" && loginStatus !== "failed");
  const uploadButtonLabel = hasUploadedQR ? "更新上传二维码" : "上传 BWS 二维码";

  useEffect(() => {
    if (user?.id && !userSelectedSourceRef.current) setSourceMode(user.qrSource ?? "uploaded");
  }, [user?.id, user?.qrSource]);

  const uploadQR = useMutation({
    mutationFn: async (file: File) => {
      const form = new FormData();
      form.append("file", file);
      await api("/me/qr/upload", { method: "POST", body: form });
    },
    onError: (err) => setError(err instanceof Error ? err.message : "上传二维码失败"),
    onSuccess: async () => {
      setError("");
      userSelectedSourceRef.current = false;
      setSourceMode("uploaded");
      setQRReloadNonce((value) => value + 1);
      await queryClient.invalidateQueries({ queryKey: ["me"] });
    }
  });

  const deleteQR = useMutation({
    mutationFn: () => api("/me/qr/delete", { method: "POST" }),
    onError: (err) => setError(err instanceof Error ? err.message : "删除二维码失败"),
    onSuccess: async () => {
      setError("");
      setQRReloadNonce((value) => value + 1);
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

  const changeQRSource = useMutation({
    mutationFn: setQRSource,
    onError: (err) => setError(err instanceof Error ? err.message : "切换二维码来源失败"),
    onSuccess: async (data) => {
      setError("");
      userSelectedSourceRef.current = false;
      setSourceMode(data.user.qrSource);
      setQRReloadNonce((value) => value + 1);
      queryClient.setQueryData(["me"], data);
      await queryClient.invalidateQueries({ queryKey: ["me"] });
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
        userSelectedSourceRef.current = true;
        setSourceMode("bilibili_generated");
        if (currentSource !== "bilibili_generated") {
          await changeQRSource.mutateAsync("bilibili_generated");
        }
        await queryClient.invalidateQueries({ queryKey: ["me"] });
      }
    }
  });

  const unbindAccount = useMutation({
    mutationFn: unbindBilibiliAccount,
    onError: (err) => setError(err instanceof Error ? err.message : "退出 B 站登录失败"),
    onSuccess: async () => {
      setError("");
      setLoginQR(null);
      setLoginStatus(undefined);
      userSelectedSourceRef.current = false;
      setSourceMode("uploaded");
      setQRReloadNonce((value) => value + 1);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["bilibili-account"] }),
        queryClient.invalidateQueries({ queryKey: ["me"] })
      ]);
    }
  });

  function selectSource(source: "uploaded" | "bilibili_generated") {
    userSelectedSourceRef.current = true;
    setSourceMode(source);
    if (source === "uploaded") {
      if (currentSource !== "uploaded") changeQRSource.mutate("uploaded");
      return;
    }
    if (isBound && currentSource !== "bilibili_generated") {
      changeQRSource.mutate("bilibili_generated");
    }
  }

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
          </Stack>
        </Stack>

        {error && <Alert severity="error">{error}</Alert>}

        <Box
          data-testid="profile-workbench"
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", lg: "360px minmax(0, 1fr)" },
            gap: 3,
            alignItems: "stretch"
          }}
        >
          <Stack spacing={2}>
            <Paper variant="outlined" sx={{ borderRadius: panelRadius, p: 2, bgcolor: "background.paper" }}>
              <Stack spacing={1.25}>
                <Typography component="h2" variant="h6" sx={{ fontWeight: 900 }}>
                  二维码来源
                </Typography>
                <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 0.75, p: 0.5, border: 1, borderColor: "divider", borderRadius: 999, bgcolor: "background.default" }}>
                  <Button
                    variant={sourceMode === "uploaded" ? "contained" : "text"}
                    disabled={changeQRSource.isPending}
                    onClick={() => selectSource("uploaded")}
                    sx={{ borderRadius: 999 }}
                  >
                    上传图片
                  </Button>
                  <Button
                    variant={sourceMode === "bilibili_generated" ? "contained" : "text"}
                    disabled={changeQRSource.isPending}
                    onClick={() => selectSource("bilibili_generated")}
                    sx={{ borderRadius: 999 }}
                  >
                    B 站生成
                  </Button>
                </Box>
              </Stack>
            </Paper>

            <Paper variant="outlined" sx={{ borderRadius: panelRadius, p: 2, bgcolor: "background.paper" }}>
              <Stack spacing={1.25}>
                <Typography component="h2" variant="h6" sx={{ fontWeight: 900 }}>
                  账号绑定
                </Typography>
                {providers.length > 0 ? (
                  <Stack spacing={1}>
                    {providers.map((provider) => {
                      const linked = linkedAccounts.find((item) => item.providerId === provider.id);
                      return (
                        <Box
                          key={provider.id}
                          sx={{
                            display: "grid",
                            gridTemplateColumns: "minmax(0, 1fr) auto",
                            alignItems: "center",
                            gap: 1,
                            px: 1.25,
                            py: 1,
                            borderRadius: controlSurfaceRadius,
                            bgcolor: "action.hover"
                          }}
                        >
                          <Box sx={{ minWidth: 0 }}>
                            <Typography sx={{ fontWeight: 850 }} noWrap>
                              {provider.name}
                            </Typography>
                            {linked?.displayName && (
                              <Typography color="text.secondary" sx={{ fontSize: 13 }} noWrap>
                                {linked.displayName}
                              </Typography>
                            )}
                          </Box>
                          {linked ? (
                            <Chip size="small" color="success" label="已绑定" />
                          ) : (
                            <Button size="small" variant="outlined" href={`/auth/oauth/${provider.id}/login`} sx={{ borderRadius: 999 }}>
                              绑定 {provider.name}
                            </Button>
                          )}
                        </Box>
                      );
                    })}
                  </Stack>
                ) : (
                  <Typography color="text.secondary" sx={{ fontSize: 14 }}>
                    暂无可绑定渠道
                  </Typography>
                )}
              </Stack>
            </Paper>

            {sourceMode === "bilibili_generated" ? (
              <Paper variant="outlined" sx={{ borderRadius: panelRadius, p: { xs: 2, sm: 2.5 }, bgcolor: "background.paper" }}>
                <Stack spacing={2}>
                  <Stack direction="row" sx={{ alignItems: "flex-start", justifyContent: "space-between", gap: 1.5 }}>
                    <Box>
                      <Typography variant="h6" component="h2" sx={{ fontWeight: 900 }}>
                        B 站扫码登录
                      </Typography>
                      <Typography color="text.secondary" sx={{ fontSize: 14, mt: 0.5 }}>
                        扫码后可使用 B 站账号生成 BWS 二维码。
                      </Typography>
                    </Box>
                    {isBound && (
                      <Avatar data-testid="bilibili-account-avatar" src={account?.faceUrl} sx={{ width: 42, height: 42 }}>
                        {account?.uname?.[0] ?? user?.displayName?.[0]}
                      </Avatar>
                    )}
                  </Stack>

                  {isBound && (
                    <Box sx={{ minWidth: 0, px: 1.5, py: 1.25, borderRadius: controlSurfaceRadius, bgcolor: "action.hover" }}>
                      <Typography sx={{ fontWeight: 850 }} noWrap>
                        {account?.uname}
                      </Typography>
                      <Typography color="text.secondary" sx={{ fontSize: 13 }} noWrap>
                        B 站账号已登录
                      </Typography>
                    </Box>
                  )}

                  <Box
                    sx={{
                      display: "grid",
                      placeItems: "center",
                      minHeight: loginQRImage ? 252 : 148,
                      border: 1,
                      borderColor: loginQRMissingImage ? "error.main" : "divider",
                      borderRadius: panelRadius,
                      bgcolor: "background.default",
                      p: 2
                    }}
                  >
                    {loginQRImage ? (
                      <Box
                        component="img"
                        src={loginQRImage}
                        alt="B 站登录二维码"
                        sx={{ width: 220, height: 220, maxWidth: "100%", borderRadius: "12px", objectFit: "contain", bgcolor: "#fff", p: 1 }}
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
                    <Alert severity="error" variant="outlined" sx={{ borderRadius: panelRadius }}>
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
            ) : (
              <Paper variant="outlined" sx={{ borderRadius: panelRadius, p: 2, bgcolor: "background.paper" }}>
                <Stack spacing={1.5}>
                  <Box>
                    <Typography variant="h6" component="h2" sx={{ fontWeight: 900 }}>
                      上传二维码
                    </Typography>
                    <Typography color="text.secondary" sx={{ fontSize: 14, mt: 0.5 }}>
                      不登录 B 站时可继续使用上传图片。
                    </Typography>
                  </Box>
                  <Button component="label" variant="outlined" startIcon={<CloudUploadIcon />} disabled={uploadQR.isPending}>
                    {uploadButtonLabel}
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
            )}
          </Stack>

          <Box
            data-testid="current-qr-preview"
            sx={{
              border: 1,
              borderColor: "divider",
              borderRadius: panelRadius,
              p: { xs: 2, sm: 3 },
              bgcolor: "background.paper",
              minHeight: { xs: 520, lg: 680 },
              display: "flex",
              flexDirection: "column",
              gap: 2.5
            }}
          >
            <Typography variant="h6" sx={{ fontWeight: 900 }}>
              当前互助二维码
            </Typography>

            {qrSrc ? (
              <Box
                component="img"
                src={qrSrc}
                alt="我的二维码"
                sx={{
                  width: "min(100%, 620px)",
                  maxHeight: { xs: 440, md: 620 },
                  objectFit: "contain",
                  borderRadius: "12px",
                  alignSelf: "center",
                  m: "auto"
                }}
              />
            ) : (
              <Stack sx={{ flex: 1, alignItems: "center", justifyContent: "center", gap: 1, color: "text.secondary", textAlign: "center", px: 2 }}>
                <Typography variant="h6" sx={{ fontWeight: 850, color: "text.primary" }}>
                  暂无可用二维码
                </Typography>
                <Typography sx={{ maxWidth: 360 }}>
                  先完成 B 站扫码登录，或上传一张 BWS 二维码图片。
                </Typography>
              </Stack>
            )}
          </Box>
        </Box>
      </Stack>
    </UserLayout>
  );
}
