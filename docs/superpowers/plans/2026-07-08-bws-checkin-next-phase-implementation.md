# BWS Checkin 下一阶段实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 按 A、B、C 顺序补齐现场稳定性、组管理运营和生产安全能力。

**架构：** 在现有 Go + SQLite + React 架构上增量实现。后端先补数据模型、权限校验和 API；前端再接入对应 UI；PWA 同步继续后台自动运行，只显示离线模式徽标。

**技术栈：** Go、Chi、SQLite、React、TypeScript、MUI、TanStack Query、PWA Cache Storage。

---

## 文件结构

后端：

- 修改：`backend/internal/store/schema.sql`，增加组状态字段和审计日志表。
- 修改：`backend/internal/store/store.go`，增加组更新、锁定、归档、审计日志和二维码旧路径处理。
- 修改：`backend/internal/store/store_test.go`，覆盖数据层新行为。
- 修改：`backend/internal/http/handlers.go`，增加动作式 API、权限校验、归档限制和上传校验。
- 修改：`backend/internal/http/handlers_test.go`，覆盖 API 行为。
- 修改：`backend/internal/http/session.go`，增加签名 Session 和 Cookie 配置。
- 修改：`backend/internal/config/config.go`，增加生产安全配置校验。
- 修改：`backend/internal/filestore/local.go`，支持删除旧二维码。

前端：

- 修改：`frontend/src/api/types.ts`，补充归档、锁定和二维码状态字段。
- 修改：`frontend/src/features/home/HomePage.tsx`，增加归档组开关。
- 修改：`frontend/src/features/groups/GroupPage.tsx`，增加极简离线徽标、归档禁用、自动切换下一未完成成员和成员二维码状态。
- 修改：`frontend/src/features/groups/GroupDialogs.tsx`，增加组编辑弹窗。
- 修改：`frontend/src/offline/groupSnapshot.ts`，维护完整快照结构。
- 修改：`frontend/src/styles.css`，补充归档、离线和缺失二维码状态样式。

文档：

- 修改：`README.md`，补充生产配置和备份说明。

## 任务 1：后端组状态与权限

- [ ] **步骤 1：编写失败测试**

在 `backend/internal/store/store_test.go` 增加测试，覆盖：

- `UpdateGroup` 修改名称、日期和说明。
- `SetGroupJoinLocked` 后 `JoinGroup` 返回锁定错误。
- `ArchiveGroup` 后 `JoinGroup` 和任务状态修改返回归档错误。

运行：

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/store
```

预期：FAIL，报错包含缺失方法。

- [ ] **步骤 2：实现数据模型和 store 方法**

修改 `schema.sql` 和 `store.go`：

- `groups` 当前 schema 包含 `join_locked` 和 `archived_at`。
- 增加 `UpdateGroup`、`SetGroupJoinLocked`、`ArchiveGroup`。
- `JoinGroup` 和 `SyncTaskCompletion` 检查归档 / 锁定状态。

- [ ] **步骤 3：运行数据层测试**

运行：

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/store
```

预期：PASS。

- [ ] **步骤 4：Commit**

```powershell
git add backend/internal/store backend/internal/domain
git commit -m "feat(后端): 添加互助组状态管理"
```

## 任务 2：后端组管理 API

- [ ] **步骤 1：编写失败测试**

在 `backend/internal/http/handlers_test.go` 增加测试，覆盖：

- 创建者可调用 `/group/update`、`/group/join-lock`、`/group/join-unlock`、`/group/archive`。
- 普通成员调用上述接口返回 `owner_role_required`。
- 归档组不允许继续完成任务。

运行：

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/http
```

预期：FAIL，接口返回 404 或缺失字段。

- [ ] **步骤 2：实现 API**

在 `/api/v1` 下注册：

- `POST /group/update`
- `POST /group/join-lock`
- `POST /group/join-unlock`
- `POST /group/archive`

更新 `GET /group/list`，支持 `includeArchived=1`。

- [ ] **步骤 3：运行 API 测试**

运行：

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/http
```

预期：PASS。

- [ ] **步骤 4：Commit**

```powershell
git add backend/internal/http backend/internal/store backend/internal/domain
git commit -m "feat(后端): 添加互助组管理接口"
```

## 任务 3：前端组管理与归档体验

- [ ] **步骤 1：更新类型和页面行为**

更新前端类型，增加：

- `joinLocked`
- `archivedAt`
- `qrMissing`

首页增加「显示已归档」开关，默认隐藏归档组。

- [ ] **步骤 2：实现创建者管理 UI**

在创建者更多菜单中增加：

- 编辑组信息。
- 锁定 / 解锁加入。
- 归档互助组。
- 移除成员二次确认。

- [ ] **步骤 3：实现归档和离线状态 UI**

互助组详情页：

- 离线时只显示「离线模式」徽标。
- 归档时显示「已归档」徽标。
- 归档后禁用完成 / 撤销按钮。
- 成员缺失二维码时在成员格中显示状态。
- 完成当前成员后自动切到下一个未完成成员。

- [ ] **步骤 4：运行前端构建**

运行：

```powershell
cd frontend
pnpm build
```

预期：PASS。

- [ ] **步骤 5：Commit**

```powershell
git add frontend/src
git commit -m "feat(前端): 完善互助组管理体验"
```

## 任务 4：生产安全与二维码校验

- [ ] **步骤 1：编写失败测试**

在后端测试中覆盖：

- 非图片内容上传被拒绝。
- 替换二维码删除旧文件。
- 生产环境关闭 mock 登录且缺少 OIDC 配置时配置校验失败。

运行：

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./...
```

预期：FAIL。

- [ ] **步骤 2：实现安全配置和上传校验**

更新：

- `config.Load` 和配置校验。
- Session Cookie 签名和安全配置。
- 上传时解码 PNG、JPEG、WebP 并限制大小。
- 替换 / 删除二维码时清理旧文件。

- [ ] **步骤 3：运行后端测试**

运行：

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./...
```

预期：PASS。

- [ ] **步骤 4：Commit**

```powershell
git add backend
git commit -m "feat(后端): 增强生产安全配置"
```

## 任务 5：审计日志与文档

- [ ] **步骤 1：编写失败测试**

在 store 和 API 测试中断言关键动作写入 `audit_logs`。

运行：

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./internal/store ./internal/http
```

预期：FAIL，缺少审计日志记录。

- [ ] **步骤 2：实现审计日志**

实现 `AppendAuditLog` 和测试需要的查询 helper，在以下动作写入日志：

- 组更新、锁定、解锁、归档。
- 成员移除。
- 任务完成 / 撤销。
- 二维码上传 / 删除。

- [ ] **步骤 3：更新 README**

补充：

- 生产环境变量。
- Cookie 配置说明。
- SQLite 和 uploads 的备份要求。

- [ ] **步骤 4：运行完整验证**

运行：

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./...
```

```powershell
cd frontend
pnpm build
```

```powershell
git diff --check
```

预期：全部通过。

- [ ] **步骤 5：Commit**

```powershell
git add backend README.md
git commit -m "feat(审计): 记录关键业务操作"
```

## 自检结果

- A 现场稳定性由任务 3 覆盖：离线徽标、归档禁用、缺二维码状态、自动切换下一未完成成员。
- B 组管理由任务 1、任务 2、任务 3 覆盖：编辑、锁定、解锁、归档、成员移除确认。
- C 生产安全由任务 4、任务 5 覆盖：Session、OIDC 配置校验、上传校验、旧文件删除、审计日志和备份文档。
- 同步 UI 不包含手动同步、同步队列详情或冲突提示，符合当前产品约束。
