# 更新日志

## v0.2.0 - 2026-07-10

真实数据与生产登录版本。

### 功能

- 支持 B 站扫码绑定账号，后端加密保存 Cookie，并可生成 BWS 互助二维码。
- 支持从 BWS 接口同步乐园任务，内置 7 月 10 日、11 日、12 日三天默认点位数据。
- 支持 Live 打卡状态刷新，区分手动状态和官方接口状态。
- 支持多 OAuth provider 登录和账号绑定，当前可配置 QQ OAuth 与 OIDC provider。
- 后端路由统一收敛到 `/api/v1/*`，健康检查改为 `/api/v1/healthz`。
- 个人中心、二维码来源切换、扫码登录工作台和点位选择弹窗完成上线前体验整理。
- 新增 `deploy/production.env.example`，提供 QQ OAuth 与 Casdoor OIDC 的线上配置模板。

### 发布验证

- `backend`: `go test ./...` 通过。
- `backend`: `go vet ./...` 通过。
- `backend`: `gofmt -l .` 无输出。
- `backend`: `go build -trimpath -ldflags="-s -w"` 可生成 Windows x64 二进制。
- `frontend`: `pnpm test` 通过，9 个测试文件、22 个测试。
- `frontend`: `pnpm build` 通过；存在 Vite 大 chunk 警告，不阻塞发布。
- `root`: `git diff --check` 通过。

### 已知限制

- 发布工作流仍需要推送 `v0.2.0` tag 后由 GitHub Actions 生成正式 Release 产物。
- B 站 Cookie 密钥仍是单密钥模型，轮换会影响已绑定账号的后续接口请求。

## v0.1.0 - 2026-07-09

首个 MVP 发布版本。

### 功能

- 支持 OIDC 生产登录和本地 mock 登录。
- 支持用户上传、更新和删除自己的互助二维码。
- 支持创建、加入、编辑、锁定加入和归档互助组。
- 支持全屏二维码打卡视图、成员切换、点位切换和任务完成状态记录。
- 支持点位分组、点位图标、标题、乐园币奖励和描述。
- 支持 PWA 离线快照、二维码图片预缓存和离线完成队列同步。
- 支持单个 Go 二进制内嵌前端 `dist` 发布。

### 发布验证

- `backend`: `go test ./...` 通过。
- `backend`: `go vet ./...` 通过。
- `backend`: `gofmt -l .` 无输出。
- `frontend`: `pnpm test` 通过，8 个测试文件、10 个测试。
- `frontend`: `pnpm build` 通过；存在 Vite 大 chunk 警告，不阻塞发布。
- `root`: `git diff --check` 通过。

### 已知限制

- 任务列表仍使用内置默认点位，不接入真实 BWS 接口。
- 打卡状态仍以互助组内手动状态为准，不读取 BWS 线上状态。
- 二维码来源仅支持用户上传，不支持 B 站账号登录后自动生成。
