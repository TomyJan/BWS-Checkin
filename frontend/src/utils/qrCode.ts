import qrcode from "qrcode-generator";

export function qrCodeDataURL(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return "";

  const qr = qrcode(0, "M");
  qr.addData(trimmed);
  qr.make();
  return qr.createDataURL(8, 2);
}
