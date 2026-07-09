# BWS 真实数据版本实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 在保留手动互助路径的同时，接入 B 站账号绑定、任务同步、BWS 二维码生成和 Live 打卡状态刷新。

**架构：** 前端仍只访问本站 `/api/v1/*`。后端新增 B 站适配层、任务同步服务和二维码生成模块，把不稳定的外部接口隔离在后端，站内业务只使用稳定的任务、账号、二维码来源和完成状态模型。

**技术栈：** Go 1.25、Chi、SQLite、React、Vite、MUI、TanStack Query、PWA Cache Storage、pnpm。

---

## 文件结构

- 修改：`backend/internal/store/schema.sql`，扩展用户、任务、完成状态和同步元数据表。
- 修改：`backend/internal/domain/types.go`，新增二维码来源、B 站账号摘要、完成状态枚举和任务外部字段。
- 修改：`backend/internal/store/store.go`，新增账号、二维码来源、任务同步和状态写入查询。
- 测试：`backend/internal/store/store_test.go`，覆盖 schema 初始化、二维码来源、账号保存和完成状态转换。
- 创建：`backend/internal/qrcode/bws.go`，生成 BWS 个人二维码内容和 PNG。
- 测试：`backend/internal/qrcode/bws_test.go`，覆盖 AES-CBC 加密、URL 生成和 PNG 输出。
- 创建：`backend/internal/bilibili/client.go`，封装 Passport 扫码登录、`nav` 和 BWS 任务接口。
- 创建：`backend/internal/bilibili/crypto.go`，加密保存 Cookie Jar JSON。
- 测试：`backend/internal/bilibili/client_test.go`，使用 `httptest.Server` 覆盖扫码状态、Cookie 提取、用户信息和任务列表解析。
- 创建：`backend/internal/tasksync/syncer.go`，封装启动刷新、定时刷新、按需刷新和手动刷新。
- 测试：`backend/internal/tasksync/syncer_test.go`，覆盖成功写入、失败保留旧数据和 TTL 后台刷新。
- 修改：`backend/internal/config/config.go`，新增 BWS/Bilibili 配置项。
- 修改：`backend/internal/app/app.go`，注入 B 站客户端、二维码服务和任务同步服务。
- 修改：`backend/internal/http/router.go`，新增动作式 API 路由。
- 修改：`backend/internal/http/handlers.go`，接入账号绑定、二维码来源、任务同步和 Live 状态刷新。
- 测试：`backend/internal/http/handlers_test.go`，覆盖新 API 的鉴权、业务错误 envelope 和状态规则。
- 修改：`frontend/src/api/types.ts`，同步新增账号、二维码来源、任务外部字段和完成状态类型。
- 修改：`frontend/src/api/client.ts`，集中封装新 API 调用。
- 修改：`frontend/src/features/profile/ProfilePage.tsx`，重写个人中心账号绑定与二维码来源管理。
- 测试：`frontend/src/features/profile/ProfilePage.test.tsx`，覆盖绑定流程状态展示、来源切换和预览。
- 修改：`frontend/src/features/groups/GroupPage.tsx`，按状态来源切换手动按钮和刷新按钮。
- 修改：`frontend/src/offline/completionSync.ts`，只同步手动状态，保留 Live 快照只读。
- 修改：`frontend/src/offline/groupSnapshot.ts`，缓存完成状态枚举和 Live 元信息。
- 测试：`frontend/src/features/groups/GroupPage.test.tsx` 和 `frontend/src/offline/completionSync.test.ts`，覆盖按钮逻辑和离线队列规则。
- 修改：`README.md` 和 `docs/superpowers/specs/2026-07-09-bws-live-data-design.md`，补充配置、接口和验证说明。

## 任务 1：后端数据模型基础

**文件：**
- 修改：`backend/internal/store/schema.sql`
- 修改：`backend/internal/domain/types.go`
- 修改：`backend/internal/store/store.go`
- 测试：`backend/internal/store/store_test.go`

- [ ] **步骤 1：编写失败的 store 测试**

新增测试覆盖：

```go
func TestStoreBilibiliAccountAndQRSource(t *testing.T) {
	store := newTestStore(t)
	user := createTestUser(t, store, "u1")

	account := domain.BilibiliAccount{
		UserID:           user.ID,
		MID:              "123456",
		Uname:            "bws-user",
		FaceURL:          "https://example.com/face.png",
		CookieCiphertext:  "ciphertext",
		CookieExpiresAt:   time.Now().Add(time.Hour),
		LastValidatedAt:   time.Now(),
	}

	require.NoError(t, store.SaveBilibiliAccount(context.Background(), account))
	got, err := store.BilibiliAccount(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, "123456", got.MID)

	require.NoError(t, store.SetUserQRSource(context.Background(), user.ID, domain.QRSourceBilibiliGenerated))
	me, err := store.UserByID(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, domain.QRSourceBilibiliGenerated, me.QRSource)
}
```

```go
func TestStoreCompletionStatusKeepsLiveAuthoritative(t *testing.T) {
	store := newTestStore(t)
	group, task, target, actor := createCompletionFixture(t, store)

	liveTime := time.Now().Add(-time.Minute)
	require.NoError(t, store.UpsertLiveTaskCompletion(context.Background(), store.LiveCompletionInput{
		GroupID: group.ID, TaskID: task.ID, TargetUserID: target.ID,
		Status: domain.CompletionStatusLiveCompleted, CheckedAt: liveTime,
	}))

	err := store.SyncTaskCompletion(context.Background(), group.ID, task.ID, target.ID, actor.ID, false, time.Now())
	require.ErrorIs(t, err, store.ErrLiveCompletionLocked)
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/store -run "TestStoreBilibiliAccountAndQRSource|TestStoreCompletionStatusKeepsLiveAuthoritative"
```

预期：失败，报错包含 `undefined: domain.BilibiliAccount`、`SaveBilibiliAccount` 或 `ErrLiveCompletionLocked`。

- [ ] **步骤 3：实现最少数据模型**

实现：

- `users.qr_source TEXT NOT NULL DEFAULT 'uploaded' CHECK (...)`。
- `bilibili_accounts` 表。
- `tasks` 增加 `external_id`、`image_url`、`venue_id`、`venue_name`、`event_day`、`sync_source`。
- `task_completions` 使用 `status`、`source`、`live_checked_at`，`completed` 继续作为当前查询需要的布尔字段，开发期允许重建数据库。
- `task_sync_state` 表保存最近同步结果。
- `domain.User.QRSource`、`domain.BilibiliAccount`、`domain.CompletionStatus`、`domain.CompletionSource`。
- `SaveBilibiliAccount`、`BilibiliAccount`、`UnbindBilibiliAccount`、`SetUserQRSource`、`UpsertLiveTaskCompletion`。

- [ ] **步骤 4：运行测试验证通过**

运行：

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/store
```

预期：`ok`。

- [ ] **步骤 5：原子提交**

```powershell
git add backend/internal/store/schema.sql backend/internal/domain/types.go backend/internal/store/store.go backend/internal/store/store_test.go
git commit -m "feat(后端): 扩展真实数据基础模型"
```

## 任务 2：BWS 二维码生成

**文件：**
- 创建：`backend/internal/qrcode/bws.go`
- 测试：`backend/internal/qrcode/bws_test.go`
- 修改：`backend/go.mod`
- 修改：`backend/go.sum`

- [ ] **步骤 1：编写失败的二维码测试**

```go
func TestBWSQRCodeURLIsStable(t *testing.T) {
	url, err := qrcode.BWSURL("123456")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(url, "https://www.bilibili.com/blackboard/era/bws2026-live.html?key="))

	again, err := qrcode.BWSURL("123456")
	require.NoError(t, err)
	require.Equal(t, url, again)
}

func TestBWSQRCodePNG(t *testing.T) {
	png, err := qrcode.BWSPNG("123456")
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(png, []byte{0x89, 'P', 'N', 'G'}))
}
```

- [ ] **步骤 2：运行测试验证失败**

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/qrcode
```

预期：失败，报错包含 `package .../internal/qrcode is not in std` 或 `undefined`。

- [ ] **步骤 3：实现最少二维码模块**

实现：

- AES-CBC + PKCS#7 padding。
- key：`f2CmYe*nls&MW*75`。
- iv：`7VKmLf4NvGO#83Y@`。
- URL 使用 `url.QueryEscape`。
- PNG 生成使用 `github.com/skip2/go-qrcode` 或同等轻量库。

- [ ] **步骤 4：运行测试验证通过**

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/qrcode
```

预期：`ok`。

- [ ] **步骤 5：原子提交**

```powershell
git add backend/internal/qrcode backend/go.mod backend/go.sum
git commit -m "feat(后端): 添加 BWS 二维码生成"
```

## 任务 3：B 站客户端与 Cookie 加密

**文件：**
- 创建：`backend/internal/bilibili/client.go`
- 创建：`backend/internal/bilibili/crypto.go`
- 测试：`backend/internal/bilibili/client_test.go`

- [ ] **步骤 1：编写失败的客户端测试**

测试内容：

- `CreateLoginQRCode` 解析 `url` 和 `qrcode_key`。
- `PollLoginQRCode` 把 B 站状态码映射为 `pending_scan`、`pending_confirm`、`expired`、`confirmed`、`failed`。
- 扫码成功时从 Cookie Jar 提取 Cookie，并通过 `Nav` 解析 `mid`、`uname`、`face`。
- `EncryptCookieJar` 和 `DecryptCookieJar` 能往返，密文不包含明文 Cookie 名值。

- [ ] **步骤 2：运行测试验证失败**

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/bilibili
```

预期：失败，报错包含缺失包或缺失类型。

- [ ] **步骤 3：实现最少客户端**

实现：

- 客户端 base URL 可注入，生产默认 `https://passport.bilibili.com` 和 `https://api.bilibili.com`。
- Cookie 只保存在后端内存 Jar 和加密后的 JSON 中。
- 加密使用 AES-GCM，密钥来自配置，经 SHA-256 派生 32 字节 key。
- 错误类型只暴露稳定错误码，不返回原始响应体。

- [ ] **步骤 4：运行测试验证通过**

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/bilibili
```

预期：`ok`。

- [ ] **步骤 5：原子提交**

```powershell
git add backend/internal/bilibili
git commit -m "feat(后端): 添加 B 站登录客户端"
```

## 任务 4：账号绑定与二维码来源 API

**文件：**
- 修改：`backend/internal/config/config.go`
- 修改：`backend/internal/app/app.go`
- 修改：`backend/internal/http/router.go`
- 修改：`backend/internal/http/handlers.go`
- 测试：`backend/internal/http/handlers_test.go`

- [ ] **步骤 1：编写失败的 HTTP 测试**

覆盖：

- 未登录访问 `GET /api/v1/bilibili/account` 返回 HTTP 401。
- `POST /api/v1/bilibili/login/qrcode/create` 返回本站 envelope 和二维码内容。
- `POST /api/v1/bilibili/login/qrcode/poll` 成功后保存账号。
- `POST /api/v1/bilibili/account/unbind` 删除账号。
- `POST /api/v1/me/qr/source/set` 只能设置 `uploaded` 或 `bilibili_generated`。
- `GET /api/v1/user/qr?userId=...` 对 `bilibili_generated` 返回 PNG。

- [ ] **步骤 2：运行测试验证失败**

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/http -run "Bilibili|QRSource|UserQR"
```

预期：失败，报错包含 404 或缺失 handler。

- [ ] **步骤 3：实现最少 API**

实现动作式路由：

- `GET /api/v1/bilibili/account`
- `POST /api/v1/bilibili/login/qrcode/create`
- `POST /api/v1/bilibili/login/qrcode/poll`
- `POST /api/v1/bilibili/account/unbind`
- `POST /api/v1/me/qr/source/set`

配置：

- `BWS_BILIBILI_COOKIE_SECRET`
- `BWS_BILIBILI_LOGIN_ENABLED`
- `BWS_BILIBILI_API_BASE`
- `BWS_BILIBILI_PASSPORT_BASE`

- [ ] **步骤 4：运行测试验证通过**

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/http
```

预期：`ok`。

- [ ] **步骤 5：原子提交**

```powershell
git add backend/internal/config/config.go backend/internal/app/app.go backend/internal/http/router.go backend/internal/http/handlers.go backend/internal/http/handlers_test.go
git commit -m "feat(后端): 接入 B 站账号绑定接口"
```

## 任务 5：任务同步服务

**文件：**
- 创建：`backend/internal/tasksync/syncer.go`
- 测试：`backend/internal/tasksync/syncer_test.go`
- 修改：`backend/internal/store/store.go`
- 修改：`backend/internal/app/app.go`
- 修改：`backend/internal/http/router.go`
- 修改：`backend/internal/http/handlers.go`

- [ ] **步骤 1：编写失败的同步测试**

覆盖：

- BWS `offline/points` 成功时写入任务外部字段。
- 远端失败时保留旧任务和最近错误状态。
- 从未同步成功时仍保留 schema 内置默认任务。
- `EnsureFresh` 在 TTL 过期时触发后台刷新但不阻塞返回。

- [ ] **步骤 2：运行测试验证失败**

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/tasksync
```

预期：失败，报错包含缺失包或缺失类型。

- [ ] **步骤 3：实现最少同步服务**

实现：

- 活动 ID、日期和场馆默认值来自 `docs/bws2026-live-api.md`。
- 启动后异步刷新一次。
- 活动期间 5 分钟刷新，非活动期 1 小时刷新。
- `POST /api/v1/task/sync` 手动刷新。
- `GET /api/v1/task/sync/status` 返回最近同步状态。
- `GroupTasks` 查询前如 TTL 超过 5 分钟，触发后台刷新。

- [ ] **步骤 4：运行测试验证通过**

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/tasksync ./internal/store ./internal/http
```

预期：`ok`。

- [ ] **步骤 5：原子提交**

```powershell
git add backend/internal/tasksync backend/internal/store/store.go backend/internal/app/app.go backend/internal/http/router.go backend/internal/http/handlers.go backend/internal/tasksync/syncer_test.go
git commit -m "feat(后端): 添加 BWS 任务同步服务"
```

## 任务 6：Live 状态刷新规则

**文件：**
- 修改：`backend/internal/domain/types.go`
- 修改：`backend/internal/store/store.go`
- 修改：`backend/internal/http/router.go`
- 修改：`backend/internal/http/handlers.go`
- 测试：`backend/internal/store/store_test.go`
- 测试：`backend/internal/http/handlers_test.go`

- [ ] **步骤 1：编写失败测试**

覆盖：

- `POST /api/v1/task/complete` 遇到 Live 状态返回 `ok:false`、`error.code=live_completion_locked`。
- `POST /api/v1/task/uncomplete` 遇到 Live 状态同样被拒绝。
- `POST /api/v1/task/status/refresh` 使用目标用户 B 站账号刷新单个任务状态。
- 刷新失败不降级已有 Live 状态，只标记 `liveStale=true`。

- [ ] **步骤 2：运行测试验证失败**

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/store ./internal/http -run "LiveCompletion|StatusRefresh"
```

预期：失败，报错包含缺失路由或错误码不匹配。

- [ ] **步骤 3：实现最少状态规则**

实现：

- `MemberCompletion.status/source/liveStale/liveCheckedAt/canToggle/canRefresh`。
- 手动状态继续按 `updated_at` 新者优先。
- Live 状态优先于手动状态。
- Live 刷新只在线执行，不进入离线 pending 队列。

- [ ] **步骤 4：运行测试验证通过**

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/store ./internal/http
```

预期：`ok`。

- [ ] **步骤 5：原子提交**

```powershell
git add backend/internal/domain/types.go backend/internal/store/store.go backend/internal/http/router.go backend/internal/http/handlers.go backend/internal/store/store_test.go backend/internal/http/handlers_test.go
git commit -m "feat(后端): 支持 Live 打卡状态刷新"
```

## 任务 7：前端个人中心账号绑定

**文件：**
- 修改：`frontend/src/api/types.ts`
- 修改：`frontend/src/api/client.ts`
- 修改：`frontend/src/features/profile/ProfilePage.tsx`
- 测试：`frontend/src/features/profile/ProfilePage.test.tsx`

- [ ] **步骤 1：编写失败测试**

覆盖：

- 未绑定时显示绑定入口和上传二维码入口。
- 创建扫码登录后展示二维码。
- `pending_scan`、`pending_confirm`、`expired`、`confirmed` 映射为不同状态。
- 绑定成功后显示 B 站头像昵称和二维码来源切换。
- 切换来源后刷新预览。

- [ ] **步骤 2：运行测试验证失败**

```powershell
cd frontend
pnpm test -- ProfilePage
```

预期：失败，报错包含找不到新按钮或新 API 方法。

- [ ] **步骤 3：实现最少 UI**

实现：

- 个人中心使用现有 `UserLayout`。
- 账号绑定与二维码管理分为两个清晰区域。
- 二维码预览始终通过 `/api/v1/user/qr?userId=...`。
- 不展示系统内部 UUID。
- 深色模式使用现有 MUI theme token，不写固定浅色背景。

- [ ] **步骤 4：运行测试验证通过**

```powershell
cd frontend
pnpm test -- ProfilePage
pnpm build
```

预期：测试和构建退出码为 0。

- [ ] **步骤 5：原子提交**

```powershell
git add frontend/src/api/types.ts frontend/src/api/client.ts frontend/src/features/profile/ProfilePage.tsx frontend/src/features/profile/ProfilePage.test.tsx
git commit -m "feat(前端): 添加 B 站账号绑定界面"
```

## 任务 8：前端任务状态与离线队列

**文件：**
- 修改：`frontend/src/features/groups/GroupPage.tsx`
- 修改：`frontend/src/offline/completionSync.ts`
- 修改：`frontend/src/offline/groupSnapshot.ts`
- 测试：`frontend/src/features/groups/GroupPage.test.tsx`
- 测试：`frontend/src/offline/completionSync.test.ts`

- [ ] **步骤 1：编写失败测试**

覆盖：

- `manual_incomplete` 显示标记完成按钮。
- `manual_completed` 显示撤销完成按钮。
- `live_incomplete` 和 `live_completed` 显示刷新按钮。
- 离线时 Live 刷新按钮禁用。
- 离线队列拒绝 Live 状态变更，只保存手动状态。

- [ ] **步骤 2：运行测试验证失败**

```powershell
cd frontend
pnpm test -- GroupPage completionSync
```

预期：失败，报错包含按钮文案或队列断言不匹配。

- [ ] **步骤 3：实现最少前端行为**

实现：

- 统一从 `MemberCompletion.canToggle/canRefresh/source/status` 决定按钮。
- 刷新按钮调用 `POST /api/v1/task/status/refresh`。
- 手动切换继续走现有 complete/uncomplete 和离线 pending。
- 快照保存 `status/source/liveStale/liveCheckedAt`。

- [ ] **步骤 4：运行测试验证通过**

```powershell
cd frontend
pnpm test -- GroupPage completionSync
pnpm build
```

预期：测试和构建退出码为 0。

- [ ] **步骤 5：原子提交**

```powershell
git add frontend/src/features/groups/GroupPage.tsx frontend/src/offline/completionSync.ts frontend/src/offline/groupSnapshot.ts frontend/src/features/groups/GroupPage.test.tsx frontend/src/offline/completionSync.test.ts
git commit -m "feat(前端): 区分手动与 Live 打卡状态"
```

## 任务 9：文档、完整验证与收口

**文件：**
- 修改：`README.md`
- 修改：`docs/superpowers/specs/2026-07-09-bws-live-data-design.md`

- [ ] **步骤 1：更新文档**

补充：

- B 站绑定配置项。
- 任务同步时机。
- 二维码来源说明。
- Live 状态和离线限制。
- 生产日志和 Cookie 安全注意事项。

- [ ] **步骤 2：运行完整验证**

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./...
go vet ./...
$files = gofmt -l .; if ($files) { $files; exit 1 }
```

```powershell
cd frontend
pnpm test
pnpm build
```

```powershell
git diff --check
```

预期：全部退出码为 0。

- [ ] **步骤 3：原子提交**

```powershell
git add README.md docs/superpowers/specs/2026-07-09-bws-live-data-design.md
git commit -m "docs(真实数据): 补充接入与验证说明"
```

## 自检结果

- 规格中的账号绑定、二维码生成、任务同步、Live 状态、离线限制和日志安全要求都有对应任务。
- 每个功能任务都包含红灯测试、失败验证、最少实现、通过验证和原子提交步骤。
- 新 API 继续使用 `/api/v1`、GET / POST 和动作式路径。
- B 站 Cookie 不进入前端，不记录日志。
- 当前开发阶段只支持当前 schema，允许重建本地 SQLite。
- 前端仍通过 `frontend/src/api/client.ts` 统一访问 API。
