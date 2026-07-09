import type {
  BilibiliAccountResponse,
  BilibiliLoginPollResponse,
  BilibiliLoginQRCodeResponse,
  MeResponse,
  OAuthAccountsResponse,
  OAuthProvidersResponse
} from "./types";

const API_BASE = "/api/v1";

interface ApiEnvelope<T> {
  ok: boolean;
  data?: T;
  error?: {
    code: string;
    message: string;
  };
}

export class ApiError extends Error {
  code: string;

  constructor(code: string, message: string) {
    super(message || code);
    this.name = "ApiError";
    this.code = code;
  }
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers =
    init.body instanceof FormData
      ? init.headers
      : { "Content-Type": "application/json", ...init.headers };

  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: "include",
    headers
  });

  if (!response.ok) {
    throw new Error(`API ${response.status}`);
  }

  const envelope = (await response.json()) as ApiEnvelope<T>;
  if (!envelope.ok) {
    throw new ApiError(envelope.error?.code ?? "business_error", envelope.error?.message ?? "");
  }
  return envelope.data as T;
}

export function getMe() {
  return api<MeResponse>("/me");
}

export function getBilibiliAccount() {
  return api<BilibiliAccountResponse>("/bilibili/account");
}

export function getOAuthProviders() {
  return api<OAuthProvidersResponse>("/oauth/providers");
}

export function getOAuthAccounts() {
  return api<OAuthAccountsResponse>("/oauth/accounts");
}

export function createBilibiliLoginQRCode() {
  return api<BilibiliLoginQRCodeResponse>("/bilibili/login/qrcode/create", { method: "POST" });
}

export function pollBilibiliLoginQRCode(qrcodeKey: string) {
  return api<BilibiliLoginPollResponse>("/bilibili/login/qrcode/poll", {
    method: "POST",
    body: JSON.stringify({ qrcodeKey })
  });
}

export function unbindBilibiliAccount() {
  return api<{ ok: boolean }>("/bilibili/account/unbind", { method: "POST" });
}

export function setQRSource(source: "uploaded" | "bilibili_generated") {
  return api<MeResponse>("/me/qr/source/set", {
    method: "POST",
    body: JSON.stringify({ source })
  });
}

export function refreshTaskStatus(input: { groupId: string; taskId: string; userId: string }) {
  return api("/task/status/refresh", {
    method: "POST",
    body: JSON.stringify(input)
  });
}
