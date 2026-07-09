import "@testing-library/jest-dom/vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { ProfilePage } from "./ProfilePage";

let loginQRCodeImageDataUrl: string | undefined;

function renderProfilePage() {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false }
    }
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={["/profile"]}>
        <ProfilePage />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe("ProfilePage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    loginQRCodeImageDataUrl = "data:image/png;base64,loginqr";
    let bilibiliBound = false;
    vi.spyOn(window, "fetch").mockImplementation((input) => {
      const url = String(input);
      if (url.endsWith("/api/v1/me")) {
        return Promise.resolve(
          Response.json({
            ok: true,
            data: {
              user: {
                id: "a4fc8cfb-7dc8-485e-a270-76d18a44cdc7",
                displayName: "TomyJan",
                avatarUrl: "",
                qrImageUrl: "/api/v1/user/qr?userId=a4fc8cfb-7dc8-485e-a270-76d18a44cdc7",
                qrSource: bilibiliBound ? "bilibili_generated" : "uploaded"
              }
            }
          })
        );
      }
      if (url.endsWith("/api/v1/bilibili/account")) {
        return Promise.resolve(
          Response.json({
            ok: true,
            data: bilibiliBound
              ? { bound: true, account: { mid: "123456", uname: "BiliTomy", faceUrl: "https://example.com/face.png" } }
              : { bound: false }
          })
        );
      }
      if (url.endsWith("/api/v1/bilibili/login/qrcode/create")) {
        return Promise.resolve(
          Response.json({
            ok: true,
            data: { qrcode: { url: "https://passport.bilibili.com/qrcode", qrcodeKey: "qr-key", expiresAt: "2026-07-10T12:03:00Z", imageDataUrl: loginQRCodeImageDataUrl } }
          })
        );
      }
      if (url.endsWith("/api/v1/bilibili/login/qrcode/poll")) {
        bilibiliBound = true;
        return Promise.resolve(
          Response.json({
            ok: true,
            data: { status: "confirmed", account: { mid: "123456", uname: "BiliTomy", faceUrl: "https://example.com/face.png" } }
          })
        );
      }
      if (url.endsWith("/api/v1/me/qr/source/set")) {
        return Promise.resolve(Response.json({ ok: true, data: { user: { id: "a4fc8cfb-7dc8-485e-a270-76d18a44cdc7", displayName: "TomyJan", avatarUrl: "", qrImageUrl: "/api/v1/user/qr?userId=a4fc8cfb-7dc8-485e-a270-76d18a44cdc7", qrSource: "bilibili_generated" } } }));
      }
      return Promise.resolve(Response.json({ ok: true, data: {} }));
    });
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  test("uses the shared user navigation and presents QR management as the main task", async () => {
    renderProfilePage();

    await waitFor(() => expect(screen.getByText("BWS 互助")).toBeInTheDocument());
    expect(screen.getByRole("link", { name: "互助组" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "个人中心" })).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole("button", { name: /TomyJan/ })).toBeInTheDocument());
    expect(screen.getByRole("heading", { name: "个人中心" })).toBeInTheDocument();
    expect(screen.queryByText(/a4fc8cfb-7dc8-485e-a270-76d18a44cdc7/)).not.toBeInTheDocument();
    expect(await screen.findByRole("img", { name: "我的二维码" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "更新上传二维码" })).toBeInTheDocument();
  });

  test("supports Bilibili QR login polling and QR source switching", async () => {
    renderProfilePage();

    expect(await screen.findByRole("heading", { name: "B 站扫码登录" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "生成登录二维码" }));
    expect(await screen.findByRole("img", { name: "B 站登录二维码" })).toHaveAttribute("src", "data:image/png;base64,loginqr");
    expect(screen.queryByRole("button", { name: "检查登录状态" })).not.toBeInTheDocument();
    expect(await screen.findByText("BiliTomy")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "B 站生成" }));
    await waitFor(() => expect(screen.getByText("当前使用：B 站生成")).toBeInTheDocument());
    expect(screen.getByRole("img", { name: "我的二维码" })).toHaveAttribute("src", "/api/v1/user/qr?userId=a4fc8cfb-7dc8-485e-a270-76d18a44cdc7");
  });

  test("does not render the Bilibili login URL as an image when image data is missing", async () => {
    loginQRCodeImageDataUrl = undefined;
    renderProfilePage();

    fireEvent.click(await screen.findByRole("button", { name: "生成登录二维码" }));

    await waitFor(() => expect(screen.getByText("登录二维码暂不可用")).toBeInTheDocument());
    expect(screen.queryByRole("img", { name: "B 站登录二维码" })).not.toBeInTheDocument();
  });
});
