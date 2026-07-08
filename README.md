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
npm install
npm run dev
```

前端默认监听 `http://127.0.0.1:5173`，并通过 Vite proxy 转发 `/api`、`/auth` 和 `/uploads` 到后端。

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
npm run build
```

## 开发登录

本地开发登录由后端 `POST /api/v1/dev/login?name=TomyJan` 提供。前端登录按钮会自动调用该接口。生产环境应关闭 `BWS_DEV_AUTH` 并配置真实 OIDC。

