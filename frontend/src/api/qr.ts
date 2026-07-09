export interface QRImageUser {
  id: string;
  qrImageUrl?: string | null;
}

export function qrImageURL(user?: QRImageUser | null) {
  if (!user?.id || !user.qrImageUrl) return "";
  return `/api/v1/user/qr?userId=${encodeURIComponent(user.id)}`;
}
