import "@testing-library/jest-dom/vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { ProfilePage } from "./ProfilePage";

let loginQRCodeImageDataUrl: string | undefined;
let bilibiliPollRequests = 0;
let meQRCodeUrl: string;
let meQRSource: "uploaded" | "bilibili_generated";
let initialBilibiliBound: boolean;

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
    bilibiliPollRequests = 0;
    meQRCodeUrl = "/api/v1/user/qr?userId=a4fc8cfb-7dc8-485e-a270-76d18a44cdc7";
    meQRSource = "uploaded";
    initialBilibiliBound = false;
    let bilibiliBound = initialBilibiliBound;
    vi.spyOn(window, "fetch").mockImplementation((input, init) => {
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
                qrImageUrl: meQRCodeUrl,
                qrSource: meQRSource
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
        bilibiliPollRequests += 1;
        bilibiliBound = true;
        return Promise.resolve(
          Response.json({
            ok: true,
            data: { status: "confirmed", account: { mid: "123456", uname: "BiliTomy", faceUrl: "https://example.com/face.png" } }
          })
        );
      }
      if (url.endsWith("/api/v1/me/qr/source/set")) {
        const body = JSON.parse(String(init?.body ?? "{}")) as { source?: "uploaded" | "bilibili_generated" };
        meQRSource = body.source ?? meQRSource;
        return Promise.resolve(Response.json({ ok: true, data: { user: { id: "a4fc8cfb-7dc8-485e-a270-76d18a44cdc7", displayName: "TomyJan", avatarUrl: "", qrImageUrl: meQRCodeUrl, qrSource: meQRSource } } }));
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
    expect(screen.getByTestId("profile-workbench")).toBeInTheDocument();
    const currentQRPreview = screen.getByTestId("current-qr-preview");
    expect(currentQRPreview).toBeInTheDocument();
    expect(getComputedStyle(currentQRPreview).borderRadius).toBe("16px");
    const currentQRSurface = screen.getByTestId("current-qr-surface");
    expect(currentQRSurface).toBeInTheDocument();
    expect(getComputedStyle(currentQRSurface).borderRadius).toBe("18px");
    expect(screen.queryByTestId("current-qr-device")).not.toBeInTheDocument();
    expect(screen.queryByText(/a4fc8cfb-7dc8-485e-a270-76d18a44cdc7/)).not.toBeInTheDocument();
    const currentQRImage = await screen.findByRole("img", { name: "我的二维码" });
    expect(currentQRImage).toBeInTheDocument();
    expect(getComputedStyle(currentQRImage).backgroundColor).toBe("rgba(0, 0, 0, 0)");
    expect(screen.getByRole("button", { name: "更新上传二维码" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "替换当前图片" })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "B 站扫码登录" })).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "二维码来源" })).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "二维码来源" }).compareDocumentPosition(screen.getByRole("heading", { name: "上传二维码" })) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();
    expect(screen.queryByText("BiliTomy")).not.toBeInTheDocument();
    expect(screen.queryByText("账号已可用于生成二维码")).not.toBeInTheDocument();
  });

  test("does not claim the current QR is available when the selected source has no image URL", async () => {
    meQRCodeUrl = "";
    renderProfilePage();

    expect(await screen.findByRole("heading", { name: "当前互助二维码" })).toBeInTheDocument();
    expect(screen.getByText("缺少二维码")).toBeInTheDocument();
    expect(screen.getByText("暂无可用二维码")).toBeInTheDocument();
    expect(screen.queryByRole("img", { name: "我的二维码" })).not.toBeInTheDocument();
  });

  test("supports Bilibili QR login polling and QR source switching", async () => {
    renderProfilePage();

    expect(await screen.findByRole("heading", { name: "二维码来源" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "B 站生成" }));
    expect(await screen.findByRole("heading", { name: "B 站扫码登录" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "生成登录二维码" }));
    expect(await screen.findByRole("img", { name: "B 站登录二维码" })).toHaveAttribute("src", "data:image/png;base64,loginqr");
    expect(screen.getByText("等待 B 站客户端扫码")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "等待扫码" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "检查登录状态" })).not.toBeInTheDocument();
    expect(bilibiliPollRequests).toBe(0);
    await new Promise((resolve) => window.setTimeout(resolve, 1200));
    expect(bilibiliPollRequests).toBe(0);
    expect(screen.queryByText("BiliTomy")).not.toBeInTheDocument();
    expect(await screen.findByText("BiliTomy", undefined, { timeout: 2600 })).toBeInTheDocument();
    expect(screen.getByTestId("bilibili-account-avatar")).toBeInTheDocument();
    expect(bilibiliPollRequests).toBe(1);

    await waitFor(() => expect(screen.getByRole("img", { name: "我的二维码" })).toHaveAttribute("src", expect.stringMatching(/qr\?userId=.*&v=/)));
  });

  test("generates a renderable Bilibili login QR image when server image data is missing", async () => {
    loginQRCodeImageDataUrl = undefined;
    renderProfilePage();

    fireEvent.click(await screen.findByRole("button", { name: "B 站生成" }));
    fireEvent.click(await screen.findByRole("button", { name: "生成登录二维码" }));

    const loginImage = await screen.findByRole("img", { name: "B 站登录二维码" });
    expect(loginImage).toHaveAttribute("src", expect.stringMatching(/^data:image\//));
    expect(screen.getByText("等待 B 站客户端扫码")).toBeInTheDocument();
    expect(screen.queryByText("接口未返回二维码图片")).not.toBeInTheDocument();
  });

  test("switches visible QR source panels and reloads the current QR image", async () => {
    initialBilibiliBound = true;
    meQRSource = "bilibili_generated";
    renderProfilePage();

    const firstImage = await screen.findByRole("img", { name: "我的二维码" });
    const firstSrc = firstImage.getAttribute("src");
    expect(await screen.findByRole("heading", { name: "B 站扫码登录" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "上传二维码" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "上传图片" }));

    expect(await screen.findByRole("heading", { name: "上传二维码" })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "B 站扫码登录" })).not.toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole("img", { name: "我的二维码" }).getAttribute("src")).not.toBe(firstSrc));
    expect(screen.getByRole("img", { name: "我的二维码" })).toHaveAttribute("src", expect.stringMatching(/&v=/));
  });
});
