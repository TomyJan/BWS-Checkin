# BWS Checkin 轻量设计规格

## 背景

BWS Checkin 是 BW 乐园活动的互助打卡网站。用户登录后上传自己的二维码，加入互助组；现场用户在一个打卡点位打开该点位页面，即可横向切换所有成员二维码，帮助整组成员完成打卡。

本项目按轻量工程化实现：前后端分离，先使用 SQLite 和本地图片存储，保留后续迁移到 PostgreSQL 或对象存储的空间。

## 技术方向

- **前端：** Vite + React + TypeScript。
- **UI：** MUI Material，使用主题层实现接近 Google Material Design 3 的大圆角、浅色 / 深色自适应。
- **路由：** React Router。
- **请求状态：** TanStack Query。
- **后端：** Go + chi。
- **数据库：** SQLite，数据库访问层保持独立，避免业务代码直接散落 SQL。
- **文件存储：** MVP 使用本地目录保存二维码图片。
- **鉴权：** 生产环境只接入 OIDC；本地开发使用 mock login。

## 鉴权与用户

整站需要登录后访问。生产环境登录流程如下：

1. 前端访问未登录页面时，后端返回未登录状态。
2. 用户点击登录，跳转到 OIDC Provider。
3. OIDC callback 成功后，后端创建或更新站内用户记录。
4. 后端写入 HTTP-only Session Cookie。
5. 前端通过 `/api/v1/me` 获取当前用户资料。

用户表保存业务需要的信息：

- OIDC `subject`
- 显示名称
- 头像地址（如果 OIDC 返回）
- 二维码图片路径
- 创建时间和更新时间

生产环境不提供用户名 / 密码注册。只要 OIDC 登录成功，即允许自动注册站内用户。

本地开发模式下，后端提供 mock login，用于创建固定测试用户和切换测试身份。

## 首页

首页是登录后的第一屏，核心内容是「我的互助组」。

布局要求：

- 标题「我的互助组」和副标题位于同一块头部区域。
- 右侧只有一个加号按钮。
- 点击加号后弹出菜单，包含「创建互助组」和「加入互助组」。
- 如果用户未上传二维码，显示明确提示并引导上传；已上传时不显示「已上传二维码」状态。

互助组卡片显示：

- 组名称，例如 `BW2026 7 月 11 日`
- 组 ID，例如 `bw2026-fri`
- 成员数
- 点位数
- 当前用户角色：创建者或成员

## 创建与加入互助组

创建互助组需要填写：

- 名称
- ID
- 日期：`20260710`、`20260711`、`20260712`，界面展示为 7 月 10 日、7 月 11 日、7 月 12 日。
- 说明内容

组 ID 即邀请码。组 ID 允许自定义，但必须全站唯一。

加入互助组支持两种方式：

- 首页点击加号，选择「加入互助组」，手动输入组 ID。
- 打开邀请链接，系统自动填充组 ID，用户确认后加入。

创建者拥有管理权限：

- 复制邀请链接
- 查看成员
- 移除成员

普通成员只能查看组信息、切换任务、标记完成状态、管理自己的二维码。

## 互助组详情页

互助组详情页以二维码照片为第一优先级。进入组后默认显示当前点位和当前成员二维码。

交互模型：

- 二维码图片始终全屏填充主区域。
- 点击屏幕任意位置显示或隐藏 UI 浮层。
- UI 显示时，顶部浮层展示组名称，例如 `BW2026 7 月 11 日`。
- UI 显示时，底部浮层展示当前点位、完成进度和所有成员状态。
- UI 隐藏时，顶部和底部浮层通过动画滑出，保留二维码最大可视面积。

二维码切换：

- 左右滑动或点击左右按钮切换成员二维码。
- 当前成员在底部成员状态区高亮。
- 切换成员不会改变当前点位。

点位切换：

- 当前点位是一级上下文。
- 点击底部浮层中的点位名称，弹出完整点位列表。
- 点位列表按任务分组展示；单个点位展示名称、标题、奖励乐园币数量和描述。
- 用户从列表中选择点位后，底部成员状态区切换到该点位的数据。

## 任务与完成状态

任务按活动日期区分。系统内置 7 月 10 日、7 月 11 日、7 月 12 日三天的默认点位作为回退；接入 B 站账号后，创建者可以在互助组详情页触发当天任务同步。

状态层级如下：

1. 互助组
2. 点位任务
3. 成员在该点位下的完成状态

底部任务面板显示：

- 当前点位名称
- 完成进度，例如 `5/8`
- 所有成员在当前点位的状态

成员状态格式：

- 未完成：显示成员名和「未完成」
- 已完成：显示完成时间和打卡人，例如 `14:34 Alice`

标记完成时，记录：

- 互助组 ID
- 点位 ID
- 被打卡成员 ID
- 打卡人 ID
- 完成时间

同一个成员在同一个点位只能有一个完成记录。重复点击时不创建重复记录。

## 数据模型

核心表：

### users

- `id`
- `oidc_subject`
- `display_name`
- `avatar_url`
- `qr_image_path`
- `created_at`
- `updated_at`

### groups

- `id`
- `name`
- `day`：`friday`、`saturday`、`sunday`
- `description`
- `owner_user_id`
- `created_at`
- `updated_at`

### group_members

- `group_id`
- `user_id`
- `role`：`owner`、`member`
- `joined_at`

### tasks

- `id`
- `group_name`
- `name`
- `title`
- `reward_coins`
- `description`
- `sort_order`
- `enabled`

MVP 中任务由数据库 seed 初始化，前端始终通过 API 读取点位列表。

### task_completions

- `group_id`
- `task_id`
- `target_user_id`
- `checked_by_user_id`
- `completed`
- `completed_at`
- `updated_at`

唯一约束：`group_id + task_id + target_user_id`。

## PWA 与离线能力

前端支持 PWA。用户进入互助组详情页时，前端需要形成一个完整离线快照：

- 互助组信息。
- 全部任务和每个任务下的成员完成状态。
- 成员基本信息。
- 成员二维码图片本体，而不是只保存图片 URL。

快照保存到浏览器本地存储，二维码图片通过 Cache Storage 预缓存。断网后，用户仍可打开已经进入过的互助组，查看任务、切换成员二维码，并继续标记完成或撤销完成。

离线产生的打卡状态变动写入本地 pending 队列。网络恢复后，前端持续尝试上传。服务端保存每条状态的 `updated_at`；同一个 `group_id + task_id + target_user_id` 出现冲突时，以 `updated_at` 更新的一方为准。

## API 草案

API 统一使用 `/api/v1` 前缀。业务接口只使用 GET/POST，路径采用动作式命名，不使用 RESTful 资源路径。HTTP 状态码仅表达传输和鉴权状态：正常业务成功或业务失败大多返回 HTTP 200，通过响应体区分。

成功响应：

```json
{ "ok": true, "data": {} }
```

业务失败响应：

```json
{ "ok": false, "error": { "code": "group_id_conflict", "message": "..." } }
```

鉴权：

- `GET /api/v1/me`
- `POST /api/v1/dev/login`
- `POST /api/v1/logout`
- `GET /auth/oidc/login`
- `GET /auth/oidc/callback`

用户：

- `POST /api/v1/me/qr/upload`
- `POST /api/v1/me/qr/delete`

互助组：

- `GET /api/v1/group/list`
- `POST /api/v1/group/create`
- `GET /api/v1/group/detail?groupId=...`
- `POST /api/v1/group/join`
- `POST /api/v1/group/member/remove`

任务：

- `GET /api/v1/group/tasks?groupId=...`
- `POST /api/v1/task/complete`
- `POST /api/v1/task/uncomplete`

## 错误处理

- 未登录访问 API 返回 HTTP 401，响应体 code 为 `login_required`。
- 非成员访问互助组返回 HTTP 200，响应体 `ok: false`，code 为 `group_access_denied`。
- 非创建者移除成员返回 HTTP 200，响应体 `ok: false`，code 为 `owner_role_required`。
- 加入不存在的组返回 HTTP 200，响应体 `ok: false`。
- 创建重复组 ID 返回 HTTP 200，响应体 `ok: false`，code 为 `group_id_conflict`。
- 未上传二维码的用户在互助组详情页中显示缺失状态，不阻塞其他成员打卡。
- 上传二维码只接受常见图片格式，并限制文件大小。

## 测试重点

后端测试：

- OIDC mock 登录创建用户。
- 创建互助组时 ID 唯一约束生效。
- 加入互助组后能出现在成员列表。
- 创建者可以移除成员，普通成员不能移除成员。
- 标记完成会记录打卡人和完成时间。
- 重复标记不会创建重复完成记录。

前端测试：

- 未登录时进入登录流程。
- 首页只在未上传二维码时显示上传提示。
- 加号菜单可以打开创建和加入入口。
- 互助组详情页可以切换成员二维码。
- 点击点位名称可以打开点位列表并切换点位。
- 标记完成后成员状态和进度更新。

## 当前确认结论

- 采用前后端分离架构。
- 后端使用 Go。
- 生产环境只使用 OIDC 登录，本地开发使用 mock login。
- SQLite 和本地文件存储作为 MVP 存储方案。
- 首页操作入口收敛到右侧加号菜单。
- 互助组详情页采用全屏二维码和可隐藏浮层 UI。
- 点位是一级任务上下文，成员状态是二级信息。
- 点位切换采用「点击点位名弹出完整点位列表」。
