# BWS Checkin

BW 乐园互助打卡网站。用户通过已配置的 OAuth 渠道登录，上传自己的二维码，加入互助组后即可在一个点位为整组成员依次完成打卡记录。

## 本地开发

### 后端

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go run ./cmd/server
```

后端默认监听 `http://127.0.0.1:8080`，本地开发默认启用 mock 登录。

### 前端

```powershell
cd frontend
pnpm install
pnpm dev
```

前端默认监听 `http://127.0.0.1:5173`，并通过 Vite proxy 转发 `/api` 和 `/auth` 到后端。

### 一键开发脚本

```powershell
./scripts/dev.ps1
```

该脚本会分别启动后端和前端开发服务。

## 常用验证

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./...
```

```powershell
cd frontend
pnpm test
pnpm build
```

## 生产构建

发布产物是单个 Go 二进制，内部通过 `embed` 带上前端静态文件。线上部署时不需要单独托管 `frontend/dist`，也不需要为了前端路由额外配置反向代理。

构建顺序必须是先构建前端，再把产物复制到后端嵌入目录，最后编译后端：

```powershell
cd frontend
pnpm install --frozen-lockfile
pnpm build

cd ..
Remove-Item -Recurse -Force backend/internal/frontend/dist
Copy-Item -Recurse frontend/dist backend/internal/frontend/dist

cd backend
go build -o ../dist/bws-checkin.exe ./cmd/server
```

Release 工作流会自动执行上述流程，并发布 Linux x64 与 Windows x64 二进制。

### 发布前核对

发布当前 MVP 前，至少确认以下事项：

- 后端验证通过：`go test ./...`、`go vet ./...`，并确保 `gofmt -l .` 无输出。
- 前端验证通过：`pnpm test`、`pnpm build`。
- 生产环境关闭开发登录：`BWS_DEV_AUTH=0`。
- 生产环境至少配置 1 个 OAuth provider，旧版 `BWS_OIDC_*` 配置会自动映射为 `oidc` provider。
- 生产环境设置 `BWS_SESSION_SECRET`，并在 HTTPS 部署时设置 `BWS_COOKIE_SECURE=1`。
- 如启用 B 站账号绑定，设置 `BWS_BILIBILI_COOKIE_SECRET`，用于加密保存 B 站 Cookie。
- 备份策略同时覆盖 `BWS_DB` 和 `BWS_UPLOAD_DIR`。

## 路由约定

后端负责处理以下路径：

- `/api/v1/*`：业务 API，只使用 `GET` 和 `POST`，响应体使用业务 `ok/error` 表示业务状态。
- `/auth/oauth/*`：多 OAuth 渠道登录与回调。
- `/auth/oidc/*`：旧版 OIDC 登录与回调，保留兼容。
- `/healthz`：健康检查。

其他路径全部交给内嵌前端，用于支持 SPA 刷新和直接打开互助组页面。

二维码图片不公开暴露文件名。上传后通过鉴权 API 读取：

```text
GET /api/v1/user/qr?userId=<uuid>
```

该接口根据用户 ID 查找二维码文件，调用方必须已登录。二维码图片文件仍保存在 `BWS_UPLOAD_DIR`，但不再通过 `/uploads/*` 对外提供静态访问。

任务接口会返回点位展示元数据，包括任务分组、点位名称、标题、奖励乐园币数量和描述。前端「选择点位」弹窗按任务分组展示这些信息。

## B 站账号与真实数据

个人中心支持两种二维码来源：

- **手动上传：** 用户上传自己的 BWS 二维码图片，不需要绑定 B 站账号。
- **B 站账号生成：** 用户通过 B 站扫码登录绑定账号，后端保存加密后的 Cookie，并用绑定账号的 `mid` 生成 BWS 二维码。

后端不会把 B 站 Cookie 返回给前端。生产环境启用绑定功能时必须设置：

```powershell
$env:BWS_BILIBILI_LOGIN_ENABLED = "1"
$env:BWS_BILIBILI_COOKIE_SECRET = "replace-with-long-random-secret"
```

可选调试配置：

- `BWS_BILIBILI_LOGIN_ENABLED`：设为 `0` 可关闭 B 站扫码登录和任务同步，默认 `1`。
- `BWS_BILIBILI_PASSPORT_BASE`：覆盖 B 站 Passport 基础地址，主要用于测试。
- `BWS_BILIBILI_API_BASE`：覆盖 B 站 API 基础地址，主要用于测试。

任务列表会从 BWS 接口同步。同步策略如下：

- 服务启动后立即同步一次。
- 后台每 5 分钟同步一次。
- 用户进入互助组详情页时，如果本地任务快照超过 5 分钟未成功刷新，会异步触发同步；接口仍优先返回本地最后一次成功数据。
- `POST /api/v1/task/sync` 可触发全局同步，`GET /api/v1/task/sync/status` 可查看最近同步状态。
- 创建者可在互助组详情页右上角菜单触发 `POST /api/v1/group/task/sync`，使用该互助组创建者已绑定的 B 站 Cookie 同步任务；未绑定时会提示先完成 B 站扫码登录。

互助组活动日期使用实际日期值：

- `20260710`：7 月 10 日。
- `20260711`：7 月 11 日。
- `20260712`：7 月 12 日。

打卡状态分为手动状态和 Live 状态。手动状态可以在前端标记完成或撤销完成；Live 状态来自 BWS 接口，前端只显示「刷新状态」按钮，离线时禁用。刷新失败不会把已有 Live 状态降级为手动状态。

## 开发登录

本地开发登录由后端 `POST /api/v1/dev/login?name=TomyJan` 提供。前端登录页保留「开发登录」按钮，并同时显示后端配置的 OAuth 登录渠道。生产环境应关闭 `BWS_DEV_AUTH` 并配置真实 OAuth provider。

## OAuth 配置

生产环境关闭 mock 登录后，后端通过已配置的 OAuth provider 登录。前端会通过 `GET /api/v1/oauth/providers` 获取可用渠道，并跳转到 `/auth/oauth/<providerId>/login`。

通用配置使用 `BWS_OAUTH_PROVIDERS`，内容是 JSON 数组。字段如下：

- `id`：渠道 ID，用于 URL，例如 `qq`。
- `name`：登录页和个人中心展示名称，例如 `QQ 登录`。
- `type`：渠道类型，当前支持 `oidc` 和 `qq`；为空时按 `oidc` 处理。
- `authUrl`、`tokenUrl`、`userInfoUrl`：QQ OAuth 必填。
- `issuerUrl`：OIDC provider 必填，后端会读取 discovery 文档。
- `clientId`、`clientSecret`、`redirectUrl`：生产环境必填。

QQ 示例：

```powershell
$env:BWS_DEV_AUTH = "0"
$env:BWS_PUBLIC_BASE = "https://bws.example.com"
$env:BWS_OAUTH_PROVIDERS = '[{"id":"qq","name":"QQ 登录","type":"qq","authUrl":"https://graph.qq.com/oauth2.0/authorize","tokenUrl":"https://graph.qq.com/oauth2.0/token","userInfoUrl":"https://graph.qq.com/user/get_user_info","clientId":"your-client-id","clientSecret":"your-client-secret","redirectUrl":"https://bws.example.com/auth/oauth/qq/callback"}]'
$env:BWS_SESSION_SECRET = "replace-with-random-secret"
$env:BWS_COOKIE_SECURE = "1"
$env:BWS_COOKIE_SAMESITE = "lax"
```

旧版 OIDC 环境变量仍然可用，并会映射为一个 `oidc` provider：

```powershell
$env:BWS_DEV_AUTH = "0"
$env:BWS_PUBLIC_BASE = "https://bws.example.com"
$env:BWS_OIDC_ISSUER = "https://issuer.example.com"
$env:BWS_OIDC_CLIENT_ID = "your-client-id"
$env:BWS_OIDC_CLIENT_SECRET = "your-client-secret"
$env:BWS_OIDC_REDIRECT_URL = "https://bws.example.com/auth/oidc/callback"
$env:BWS_SESSION_SECRET = "replace-with-a-long-random-secret"
$env:BWS_COOKIE_SECURE = "1"
$env:BWS_COOKIE_SAMESITE = "lax"
```

如果不设置 `BWS_OIDC_REDIRECT_URL`，默认使用 `BWS_PUBLIC_BASE + /auth/oauth/oidc/callback`。如仍直接访问旧路径 `/auth/oidc/login`，旧回调 `/auth/oidc/callback` 也继续可用。

生产环境关闭 `BWS_DEV_AUTH` 后，服务启动时会校验 OAuth provider 和 `BWS_SESSION_SECRET`。缺少必要配置时后端会直接启动失败，避免以不完整鉴权配置对外提供服务。

个人中心会展示当前用户已绑定的 OAuth 渠道，并允许绑定未绑定渠道；当前不提供解绑入口。OAuth 登录时如果没有找到既有关联账号，后端会自动创建站内用户并落库绑定。

### Cookie 配置

- `BWS_SESSION_SECRET`：Session Cookie 签名密钥。生产环境必须设置为足够长的随机字符串。
- `BWS_COOKIE_SECURE`：设为 `1` 时只通过 HTTPS 发送 Cookie。
- `BWS_COOKIE_SAMESITE`：支持 `lax`、`strict`、`none`，默认 `lax`。
- `BWS_SESSION_MAX_AGE`：Session Cookie 有效期，单位：秒，默认 30 天。
- `BWS_BILIBILI_COOKIE_SECRET`：B 站 Cookie 加密密钥。生产环境启用 B 站登录时必须设置。

### 密钥轮换

当前版本只支持单个 `BWS_SESSION_SECRET`。轮换该密钥会让所有现有 Session Cookie 失效，用户需要重新登录。

建议做法：

- 在低峰期更新 `BWS_SESSION_SECRET`。
- 重启服务后观察结构化日志中的 `server_starting` 和请求错误情况。
- 提前告知用户需要重新登录。

如需无感轮换，需要后续扩展为「当前密钥 + 旧密钥列表」的校验模型；当前版本暂不包含该机制。

## 数据兼容性

用户 ID、成员 ID 等系统内部 ID 使用 UUID 字符串。旧版本如果已经创建过自增整型用户 ID 的 SQLite 数据库，不能直接复用新版本 schema。

升级旧本地数据前请先备份 `BWS_DB` 和 `BWS_UPLOAD_DIR`。当前项目尚未提供自动迁移脚本；开发环境可以删除旧数据库重建，生产环境需要单独编写并验证迁移脚本。

## 系统日志

后端使用结构化 JSON 日志输出到标准输出，适合由 systemd、Docker 或日志采集器统一收集。启动日志包含监听地址、数据库路径、上传目录和关键 Cookie 配置；请求日志包含方法、路径、状态码、响应字节数、耗时和来源地址。

请求日志只记录 URL path，不记录 query string、Cookie、请求体或二维码文件内容，避免把邀请码、用户 ID、Session 等敏感信息写入日志。

## 生产数据备份

当前版本使用 SQLite 和本地文件存储。生产备份时必须同时备份：

- `BWS_DB` 指向的 SQLite 数据库文件。
- `BWS_UPLOAD_DIR` 指向的二维码上传目录。

只备份数据库会丢失二维码图片；只备份上传目录会丢失用户、互助组和打卡状态。

## 离线使用

前端支持 PWA。进入互助组详情页时，会缓存组信息、任务状态、成员信息和二维码图片本体；断网后可继续查看该互助组并标记手动打卡状态。离线产生的手动打卡状态会写入本地队列，恢复网络后自动同步，服务端按更新时间较新的状态为准。

Live 状态离线时只读，不进入离线队列；恢复网络后可以通过「刷新状态」重新向服务端查询。

