# BWS Checkin

BW 乐园互助打卡网站。用户通过 OIDC 登录，上传自己的二维码，加入互助组后即可在一个点位为整组成员依次完成打卡记录。

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

## 路由约定

后端负责处理以下路径：

- `/api/v1/*`：业务 API，只使用 `GET` 和 `POST`，响应体使用业务 `ok/error` 表示业务状态。
- `/auth/oidc/*`：OIDC 登录与回调。
- `/healthz`：健康检查。

其他路径全部交给内嵌前端，用于支持 SPA 刷新和直接打开互助组页面。

二维码图片不公开暴露文件名。上传后通过鉴权 API 读取：

```text
GET /api/v1/user/qr?userId=<uuid>
```

该接口根据用户 ID 查找二维码文件，调用方必须已登录。二维码图片文件仍保存在 `BWS_UPLOAD_DIR`，但不再通过 `/uploads/*` 对外提供静态访问。

## 开发登录

本地开发登录由后端 `POST /api/v1/dev/login?name=TomyJan` 提供。前端登录按钮会自动调用该接口。生产环境应关闭 `BWS_DEV_AUTH` 并配置真实 OIDC。

## OIDC 配置

生产环境关闭 mock 登录后，后端通过 OIDC 授权码流程登录：

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

如果不设置 `BWS_OIDC_REDIRECT_URL`，默认使用 `BWS_PUBLIC_BASE + /auth/oidc/callback`。

生产环境关闭 `BWS_DEV_AUTH` 后，服务启动时会校验 OIDC 和 `BWS_SESSION_SECRET`。缺少必要配置时后端会直接启动失败，避免以不完整鉴权配置对外提供服务。

### Cookie 配置

- `BWS_SESSION_SECRET`：Session Cookie 签名密钥。生产环境必须设置为足够长的随机字符串。
- `BWS_COOKIE_SECURE`：设为 `1` 时只通过 HTTPS 发送 Cookie。
- `BWS_COOKIE_SAMESITE`：支持 `lax`、`strict`、`none`，默认 `lax`。
- `BWS_SESSION_MAX_AGE`：Session Cookie 有效期，单位：秒，默认 30 天。

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

前端支持 PWA。进入互助组详情页时，会缓存组信息、任务状态、成员信息和二维码图片本体；断网后可继续查看该互助组并标记打卡状态。离线产生的打卡状态会写入本地队列，恢复网络后自动同步，服务端按更新时间较新的状态为准。

