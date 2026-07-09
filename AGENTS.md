# AGENTS.md

## 项目定位

BWS Checkin 是 BW 乐园互助打卡网站。用户通过 OIDC 登录，上传自己的二维码，加入互助组后，即可在一个打卡点位为整组成员依次完成打卡记录。

当前优先目标是轻量 MVP：开发期前后端分离、发布期单 Go 二进制内嵌前端、Go 后端、React 前端、SQLite、本地二维码文件存储、PWA 离线可用。

## 工作入口

开始任何产品、文档或实现工作前，优先阅读：

1. `README.md`
2. `docs/superpowers/specs/2026-07-08-bws-checkin-design.md`
3. `docs/superpowers/plans/2026-07-08-bws-checkin-implementation.md`

如果任务涉及后端 API、鉴权、数据模型或离线同步，必须继续阅读：

1. `backend/internal/http/router.go`
2. `backend/internal/http/handlers.go`
3. `backend/internal/store/schema.sql`
4. `backend/internal/store/store.go`
5. `backend/internal/domain/types.go`

如果任务涉及前端页面、PWA 或离线能力，必须继续阅读：

1. `frontend/src/App.tsx`
2. `frontend/src/api/client.ts`
3. `frontend/src/api/types.ts`
4. `frontend/src/features/groups/GroupPage.tsx`
5. `frontend/src/offline/completionSync.ts`
6. `frontend/src/offline/groupSnapshot.ts`
7. `frontend/public/sw.js`
8. `frontend/public/manifest.webmanifest`

如果任务涉及 CI、依赖更新或验证命令，必须继续阅读：

1. `.github/workflows/code-check.yml`
2. `.github/dependabot.yml`
3. `frontend/package.json`
4. `backend/go.mod`

如果任务涉及发布、部署或静态资源路由，必须继续阅读：

1. `.github/workflows/release.yml`
2. `backend/internal/frontend/frontend.go`
3. `backend/internal/http/router.go`
4. `README.md`

## CodeGraph

如果仓库根目录存在 `.codegraph/`，需要理解或定位代码时，先使用 CodeGraph，再使用 `rg`、`find` 或直接读文件。

优先方式：

- MCP 工具：`codegraph_explore`
- Shell：`codegraph explore "<问题或符号名>"`

如果 CodeGraph 返回来自其他 worktree 或明显过期的结果，以当前工作区文件为准，并说明原因。

## 技能使用规则

本项目默认遵循 Superpowers 技能工作流：

- 任何新功能、行为变更或修复，先使用 `superpowers-zh:brainstorming` 明确设计边界；实现前使用 `superpowers-zh:test-driven-development`。
- 多步骤实现前，使用 `superpowers-zh:writing-plans` 或维护既有计划文档。
- 遇到 bug 或异常行为，使用 `superpowers-zh:systematic-debugging`，先复现再修复。
- 完成前必须使用 `superpowers-zh:verification-before-completion`，用新鲜命令输出证明结论。
- 编写中文文档时使用 `superpowers-zh:chinese-documentation`。
- 编写提交信息时使用 `superpowers-zh:chinese-commit-conventions`。
- 重要功能完成后，优先使用代码审查相关技能；如果当前工具环境无法派发审查代理，应进行明确自查并说明残余风险。

技能规则不覆盖用户明确指令。用户要求快速原型、跳过某流程或调整提交策略时，遵循用户指令，但仍需说明验证边界。

## 文档分层规则

- 产品方向、功能边界和当前确认结论写入 `docs/superpowers/specs/` 下的设计规格。
- 任务拆分、实现步骤和阶段验收写入 `docs/superpowers/plans/` 下的实施计划。
- 本地运行、开发登录和常用验证命令写入 `README.md`。
- 修改 API、数据库、离线同步、PWA 策略或技术栈时，同步更新设计规格和 README 中相关说明。
- 不要把远期设想混入当前 MVP 必须实现范围。需要预留时，只保留轻量扩展点。

## 技术栈规则

BWS Checkin 当前技术栈固定为：

- 后端：Go 1.25+、Chi、SQLite（`modernc.org/sqlite`）。
- 前端：Vite、React、TypeScript、pnpm。
- UI：MUI Material，主题接近 Google Material Design 3，支持浅色 / 深色自适应。
- 路由：React Router。
- 服务端状态：TanStack Query。
- PWA：Web App Manifest、Service Worker、Cache Storage。
- 本地开发鉴权：mock login。
- 生产鉴权：只接入 OIDC。
- 二维码存储：MVP 使用本地目录；不要在未明确要求时扩展对象存储。
- CI：GitHub Actions `Check Code` 工作流。
- Release：GitHub Actions 支持 tag 和手动触发，构建 Linux x64 与 Windows x64 二进制。

实现时遵循以下约束：

- 后端 API 使用 `/api/v1` 前缀。
- 业务接口只使用 `GET` 和 `POST`，不使用 `PUT`、`PATCH`、`DELETE` 表达业务动作。
- API 路径采用动作式语义，例如 `/me`、`/group/create`、`/task/complete`。
- 不使用 RESTful 资源路径表达业务动作，例如不要新增 `DELETE /groups/{id}`。
- HTTP 状态码只表达传输和鉴权状态。业务成功和业务失败大多返回 HTTP `200`，通过响应体 `ok`、`data`、`error.code` 和 `error.message` 区分。
- 未登录或登录态无效使用 HTTP `401`，用于前端进入登录流程。
- 业务错误码使用稳定字符串，例如 `group_id_conflict`、`group_access_denied`、`owner_role_required`。
- 后端 Session 使用 HTTP-only Cookie；不要引入 JWT 作为主登录会话。
- 生产环境强制使用 OIDC；本地开发使用 mock login 即可。
- 系统内部 ID（例如用户 ID、成员 ID）使用 UUID 字符串，不使用自增整数。
- SQLite schema 暂由 `backend/internal/store/schema.sql` 管理；修改表结构时必须考虑已有本地数据库的兼容迁移。
- 前端必须通过 `frontend/src/api/client.ts` 统一访问 API，不要在组件中散落重复 fetch envelope 解析。
- 前端服务端状态放在 TanStack Query，本地离线队列和快照放在明确的 `frontend/src/offline/` 模块。

## 发布与路由规则

- 最终发布产物是单个 Go 二进制，必须内嵌前端 `dist` 文件。
- 发布构建顺序是：安装前端依赖，执行 `pnpm build`，复制 `frontend/dist` 到 `backend/internal/frontend/dist`，再编译后端。
- 不需要为前端补充独立静态托管或反向代理配置。
- 后端处理 `/api/v1/*`、`/auth/oidc/*` 和 `/healthz`。
- 除上述后端路径外，其他路径全部交给内嵌前端，支持 SPA 刷新和邀请链接直达。
- 本地开发仍保留前后端分离：Vite dev server 通过 proxy 转发 `/api` 和 `/auth`。

## 二维码文件规则

- 上传文件保存在 `BWS_UPLOAD_DIR` 本地目录。
- 上传后的二维码不通过公开文件名访问，也不要暴露 `/uploads/*` 静态路径。
- 前端和离线预缓存通过鉴权 API 读取二维码：

```text
GET /api/v1/user/qr?userId=<uuid>
```

- 该接口以用户 ID 查询二维码文件；调用方必须已登录。
- 前端离线快照不能只保存二维码 URL，必须预加载二维码图片本体。

## 日志规则

- 后端系统日志使用标准库 `log/slog` 输出结构化 JSON 到标准输出。
- 启动日志应包含监听地址、数据库路径、上传目录、开发鉴权开关和关键 Cookie 配置。
- 请求日志应包含方法、URL path、HTTP 状态码、响应字节数、耗时和来源地址。
- 不要在日志中记录 query string、Cookie、请求体、OIDC token、Session secret 或二维码文件内容。
- 业务错误返回给前端时继续使用响应体 `ok/error`，不要把业务错误混入 HTTP 状态码语义。

## 核心产品约束

- 整站需要登录后访问；生产环境只使用 OIDC，本地开发可使用 mock login。
- 用户上传自己的二维码图片后，互助组成员才能帮其打卡。
- 首页核心入口是「我的互助组」，创建和加入入口收敛在右侧加号菜单。
- 互助组 ID 即邀请码，允许自定义，但必须全站唯一。
- 创建者拥有管理权限，可以邀请和移除成员；普通成员不能移除成员。
- 任务是系统内置固定点位列表，MVP 不接入线上真实任务状态。
- 互助组详情页以点位任务为一级上下文，成员状态为二级信息。
- 二维码显示是详情页第一优先级；图片应全屏填充主区域，点击屏幕显示或隐藏 UI 浮层。
- 底部任务面板显示当前点位、完成进度和所有成员状态。
- 打卡状态记录打卡人、完成时间和被打卡成员。
- 同一个 `group_id + task_id + target_user_id` 只有一个状态记录。

## PWA 与离线规则

互助组详情页必须支持断网使用。进入互助组详情页时，前端需要形成完整离线快照：

- 互助组信息。
- 全部任务信息。
- 每个任务下所有成员的完成状态。
- 成员基本信息。
- 成员二维码图片本体，而不是只保存二维码 URL。

离线实现约束：

- 组快照保存到浏览器本地存储。
- 二维码图片通过 Cache Storage 预缓存。
- Service Worker 不缓存普通 `/api/*` 响应；二维码读取 API 可作为图片资源缓存。
- 断网后，用户仍可打开已经进入过的互助组，查看任务、切换成员二维码，并继续标记完成或撤销完成。
- 离线产生的打卡状态变动写入本地 pending 队列。
- 网络恢复后，前端持续尝试同步 pending 队列。
- 服务端按 `updated_at` 处理冲突；同一成员在同一点位的状态以更新时间较新的记录为准。
- UI 只在离线时显示合适的「离线模式」提示徽标；不要增加复杂的手动同步、冲突处理或离线诊断界面，除非用户明确要求。

## 前端 UI 规则

- 保持 Google Material Design 3 方向：大圆角、清晰层级、浅色 / 深色自适应。
- 工具按钮优先使用 MUI / Material Icons 图标。
- 首页不要做营销落地页；打开后直接进入可操作的互助组管理界面。
- 详情页二维码始终是视觉中心，UI 浮层应可隐藏并带动画。
- 不要在组件中写解释性大段文案来说明功能如何使用；控件本身应足够明确。
- 移动端文字、按钮和任务面板不得溢出或互相遮挡。
- 不要把页面 section 做成层层嵌套卡片；卡片只用于列表项、弹窗和明确的工具容器。

## 测试与验证

常用验证命令：

```powershell
cd backend
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./...
```

```powershell
cd frontend
pnpm build
```

提交或声称完成前，必须至少运行与改动相关的验证：

- 后端 API、store、schema 或鉴权变更：运行 `go test ./...`。
- 前端组件、PWA、离线或类型变更：运行 `pnpm test` 和 `pnpm build`。
- CI 配置变更：检查 `.github/workflows/code-check.yml` 中的工作目录、缓存路径和脚本名称是否与当前项目一致。
- 文档-only 变更：至少运行 `git diff --check`，并人工检查 Markdown 层级和中英文排版。

不要用「应该能过」替代命令输出。验证失败时，先修复再提交。

## Git 规则

- 用户要求修改代码或文档时，完成验证后应主动按原子化边界提交；不要把多个无关变更混在一个提交里。
- 未经用户要求，不要主动合并或推送。
- 未经用户明确要求，在当前分支工作；不要主动新建分支或 Git worktree。
- 用户要求提交时，保持提交原子化；一个提交只表达一个清晰变更。
- 中文项目提交信息使用 Conventional Commits 中文适配格式，例如 `feat(前端): 添加 PWA 离线打卡能力`。
- 不要把后端、前端、CI 和文档混成一个大提交，除非用户明确要求。
- 如因用户要求创建临时 worktree，任务完成并合回目标分支后应清理该 worktree。
- 默认不 push 到远端，除非用户明确要求。
- 如果本机 GPG 签名阻塞提交，应先说明；用户要求继续时可使用 `--no-gpg-sign`。
- 不要使用 `git reset --hard`、`git checkout --` 等破坏性命令丢弃用户改动，除非用户明确要求。
- 工作区存在未提交改动时，先判断是否属于当前任务；无关改动不得回滚。

## 中文文档规范

- 中文语境使用全角标点。
- 中英文之间留空格。
- 中文与数字之间留空格。
- 技术名词、命令、路径、环境变量和 API 路径使用半角英文或代码格式。
- Markdown 标题层级不要跳级。
- 文档应直接服务产品理解和实施，不写无意义的套话。
- 修改说明时优先写清楚「当前约束」和「验证方式」，少写泛泛背景。
