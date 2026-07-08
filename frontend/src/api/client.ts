const API_BASE = "/api/v1";

interface ApiEnvelope<T> {
  ok: boolean;
  data?: T;
  error?: {
    code: string;
    message: string;
  };
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
    throw new Error(envelope.error?.code ?? "business_error");
  }
  return envelope.data as T;
}
