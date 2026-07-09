# BWS 真实数据版本设计规格

## 背景

当前版本的二维码、任务列表和打卡状态都由站内手动维护。下一阶段需要接入 B 站 BWS 2026 页面相关接口，让系统同时支持「手动互助」和「B 站账号真实数据」两种路径。

本设计基于 `docs/bws2026-live-api.md` 中从官方 BWS 页面脚本整理出的接口信息，并参考 B 站 Web 扫码登录的现行 Passport 接口资料。B 站接口不是公开稳定 API，因此实现必须把外部接口隔离在后端适配层中，避免把不稳定字段扩散到业务代码和前端。

## 目标

- 个人中心支持绑定 B 站账号，后端保存该账号 Cookie，用于后续请求 BWS 用户态接口。
- 个人中心继续支持手动上传 BWS 二维码，不强制绑定 B 站账号。
- 任务分组和任务列表从 BWS 相关 API 同步，当前内置任务作为默认回退。
- 绑定 B 站账号的用户可以由系统生成 BWS 个人二维码。
- 打卡状态区分手动状态和接口状态，并在前端呈现不同交互。
- 保持当前互助组、离线快照、二维码鉴权读取和动作式 API 约束。

## 非目标

- 不实现 BWS 官方页面完整地图、预约、抽奖和实名信息界面。
- 不直接替用户完成 BWS 打卡；官方页面目前也是跳转地图点位，不提供站外直接打卡接口。
- 不把 B 站 Cookie 暴露给前端。
- 不引入对象存储、Redis、消息队列或复杂部署方案。
- 不考虑旧开发数据兼容迁移；当前仍处于开发阶段，允许重建本地 SQLite。

## 总体方案

采用「后端可信适配层」方案。

前端只访问本站 `/api/v1/*`。后端新增 `bilibili` 适配层，负责扫码登录、Cookie Jar、BWS API 请求、二维码内容生成、任务同步和状态刷新。业务层只使用稳定的站内类型：任务、二维码来源、账号绑定状态、完成状态来源。

模块边界如下：

- `internal/bilibili`：封装 B 站 Passport 与 BWS API 客户端。
- `internal/tasksync`：封装任务同步策略和启动后台刷新。
- `internal/qrcode`：生成二维码 PNG，后端可继续通过 `GET /api/v1/user/qr?userId=...` 输出图片。
- `internal/store`：保存 B 站账号、任务快照、二维码来源和完成状态。
- `internal/http`：只暴露本站动作式 API，保持响应 envelope。

## B 站账号绑定

### 登录流程

个人中心新增「绑定 B 站账号」区域：

1. 前端调用 `POST /api/v1/bilibili/login/qrcode/create`。
2. 后端请求 B 站 Passport Web 二维码生成接口，保存一次性 `qrcode_key` 和过期时间。
3. 后端把二维码图片或二维码内容返回给前端展示。
4. 前端以 2 秒左右间隔调用 `POST /api/v1/bilibili/login/qrcode/poll`。
5. 后端轮询 B 站登录状态。成功时从响应 Cookie Jar 中提取 Cookie，并调用 `nav` 校验账号。
6. 后端保存账号绑定信息，前端刷新 `/api/v1/me`。

扫码状态映射为本站业务状态：

- `pending_scan`：未扫码。
- `pending_confirm`：已扫码，等待手机确认。
- `expired`：二维码过期。
- `confirmed`：绑定成功。
- `failed`：其他失败。

### Cookie 保存

新增表 `bilibili_accounts`：

| 字段 | 说明 |
|---|---|
| `user_id` | 站内用户 ID，主键 |
| `mid` | B 站用户 ID |
| `uname` | B 站昵称 |
| `face_url` | B 站头像 |
| `cookie_ciphertext` | 加密后的 Cookie Jar JSON |
| `cookie_expires_at` | 从 Cookie 过期时间推导的最早失效时间 |
| `refresh_token_ciphertext` | 如 Passport 返回 refresh token，则加密保存 |
| `last_validated_at` | 最近一次调用 `nav` 成功时间 |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间 |

Cookie 加密使用后端本地密钥 `BWS_BILIBILI_COOKIE_SECRET`。生产环境启用 B 站绑定功能时必须设置。日志不得记录 Cookie、二维码内容、token 或 B 站响应体。

如果 Cookie 失效，后端把账号状态标记为 `expired`，前端提示重新绑定。MVP 不做无感刷新，除非实现时确认 refresh token 刷新流程足够稳定。

## 二维码来源

用户二维码来源分为两类：

- `uploaded`：用户手动上传图片。
- `bilibili_generated`：用户绑定 B 站账号后，由后端按 BWS 页面规则生成二维码。

后端仍只暴露：

```text
GET /api/v1/user/qr?userId=<uuid>
```

读取逻辑：

1. 如果用户选择 `uploaded`，返回本地上传图片。
2. 如果用户选择 `bilibili_generated`，使用绑定账号的 `mid` 生成 BWS 二维码 PNG。
3. 如果没有可用来源，返回 `ok: false`、`qr_not_found`。

BWS 个人二维码内容规则来自官方页面脚本：使用 AES-CBC 加密 `mid`，拼接到 `https://www.bilibili.com/blackboard/era/bws2026-live.html?key=...`。后端实现时需要添加二维码生成库和 AES 生成测试。

个人中心展示：

- B 站账号绑定状态、昵称、头像。
- 当前二维码来源切换：`B 站账号生成` / `手动上传`。
- 当前二维码预览。
- 重新绑定、解绑账号、上传 / 更新二维码。

## 任务同步

### 外部来源

任务列表来自 BWS 乐园任务接口：

```text
GET /x/activity/bws/offline/points?bid=202601&fid=<fid>&day=<day>&year=202601
```

系统按场馆 ID 和活动日期拉取。场馆 ID、活动日期和活动 ID 先使用 `docs/bws2026-live-api.md` 中的正式环境默认值：

- 活动 ID：`202601`。
- 日期：`20260710`、`20260711`、`20260712`。
- 场馆：8.1 馆、1.1 馆、2.1 馆、3 馆、4.1 馆、5.1 馆、6.1 馆。

任务字段映射：

| BWS 字段 | 站内字段 |
|---|---|
| `id` | `external_id` |
| `name` | `name` |
| `image` | `image_url` |
| `unlocked` | `reward_coins` |
| `dic` | `description` |
| `fid` / 场馆 | `group_name` 或 `venue_name` |
| `day` | `event_day` |

如果地图配置可把任务点映射到展位，则补充 `venue_id`、`zone_name`、`booth_name`、`booth_code`。如果映射失败，任务仍可展示，不阻塞打卡。

### 刷新策略

刷新必须及时，但不能影响用户打开系统。

采用四层策略：

1. **启动刷新：** 服务启动后异步刷新一次任务数据，不阻塞 HTTP 服务启动。
2. **后台定时：** 活动期间每 5 分钟刷新一次；非活动期间每 1 小时刷新一次。
3. **按需刷新：** 用户进入互助组详情页时，如果任务数据超过 5 分钟未刷新，触发后台刷新；接口先返回本地最后成功快照。
4. **手动刷新：** 管理入口或开发入口提供 `POST /api/v1/task/sync`，用于现场发现任务变更时立即刷新。

失败处理：

- 刷新失败不清空现有任务。
- 如果从未成功拉取，继续使用内置默认任务。
- 保存最近一次同步结果：`last_success_at`、`last_error_at`、`last_error_code`，但日志只记录错误类别，不记录 Cookie 或完整响应。

## 完成状态模型

完成状态扩展为 4 种：

| 状态 | 含义 | 可手动切换 |
|---|---|---|
| `manual_incomplete` | 默认手动未完成 | 是 |
| `manual_completed` | 手动标记完成 | 是 |
| `live_incomplete` | BWS 接口确认未完成 | 否 |
| `live_completed` | BWS 接口确认完成 | 否 |

数据库建议将 `task_completions` 重构为：

| 字段 | 说明 |
|---|---|
| `group_id` | 互助组 ID |
| `task_id` | 站内任务 ID |
| `target_user_id` | 被打卡成员站内用户 ID |
| `status` | 上述 4 种状态 |
| `source` | `manual` 或 `live`，可由 `status` 推导但单独保存便于查询 |
| `checked_by_user_id` | 手动完成时的操作者 |
| `completed_at` | 完成时间 |
| `live_checked_at` | 最近一次从 BWS 刷新状态的时间 |
| `updated_at` | 站内冲突解决时间 |

前端行为：

- 手动状态显示「标记完成」或「撤销完成」。
- Live 状态显示「刷新状态」。
- Live 状态不可手动覆盖，除非用户解绑 B 站账号或切换回手动二维码来源。
- 离线模式下只能操作手动状态；Live 状态只显示快照，刷新按钮禁用。

冲突规则：

- 手动状态继续按 `updated_at` 新者优先。
- Live 状态优先级高于手动状态。只要目标用户绑定 B 站账号且刷新成功，该任务状态固定为 Live。
- 如果 B 站 Cookie 失效或刷新失败，不把现有 Live 状态降级为手动状态，只标记 `live_stale` 供前端提示。

## API 设计

所有接口继续使用 `/api/v1`、GET / POST 和动作式路径。

账号绑定：

- `GET /api/v1/bilibili/account`：获取当前用户 B 站绑定状态。
- `POST /api/v1/bilibili/login/qrcode/create`：创建 B 站扫码登录二维码。
- `POST /api/v1/bilibili/login/qrcode/poll`：轮询扫码登录状态。
- `POST /api/v1/bilibili/account/unbind`：解绑 B 站账号并删除 Cookie。

二维码：

- `POST /api/v1/me/qr/source/set`：切换二维码来源。
- `POST /api/v1/me/qr/upload`：保留现有上传接口。
- `POST /api/v1/me/qr/delete`：保留现有删除接口。
- `GET /api/v1/user/qr?userId=<uuid>`：保留现有鉴权读取接口。

任务同步：

- `GET /api/v1/task/sync/status`：查看最近同步状态。
- `POST /api/v1/task/sync`：手动触发任务同步。
- `GET /api/v1/group/tasks?groupId=...`：返回任务与成员状态，扩展 Live 字段。

状态刷新：

- `POST /api/v1/task/status/refresh`：刷新某个目标用户的 BWS 任务状态。
- `POST /api/v1/task/complete`：仅允许手动状态。
- `POST /api/v1/task/uncomplete`：仅允许手动状态。

`TaskStatus.members[]` 扩展字段：

```json
{
  "status": "manual_completed",
  "source": "manual",
  "liveStale": false,
  "liveCheckedAt": "2026-07-10T10:00:00Z",
  "canToggle": true,
  "canRefresh": false
}
```

## PWA 与离线

离线快照继续保存组信息、任务信息、成员状态和二维码图片本体。

新增约束：

- 快照中保存完成状态枚举、来源、`liveStale` 和 `liveCheckedAt`。
- 离线时只允许手动状态进入 pending 队列。
- Live 状态刷新必须在线执行。
- 如果某个成员二维码由 B 站账号生成，进入组详情页时仍通过 `/api/v1/user/qr` 预缓存 PNG 本体。

## 安全与日志

- B 站 Cookie 只保存在后端数据库，必须加密。
- 前端只看绑定状态，不接触 Cookie。
- 请求日志继续只记录 path，不记录 query string。
- B 站适配层日志只记录接口类别、状态码、耗时和错误码，不记录 Cookie、token、响应体、二维码内容、`mid` 加密结果。
- 解绑账号必须删除 Cookie 密文和 refresh token 密文。

## 测试策略

后端按 TDD 推进，先写失败测试：

- `bilibili` 客户端用 `httptest.Server` 模拟二维码生成、轮询、`nav`、任务列表和状态接口。
- Cookie Jar 提取和加密保存测试。
- BWS 二维码内容生成测试：同一 `mid` 生成稳定 URL，二维码接口返回 PNG。
- 任务同步测试：成功写入任务、失败保留旧任务、无远端数据时使用默认任务。
- 状态刷新测试：手动状态可切换，Live 状态不可手动覆盖，刷新失败不降级。
- API 测试：未登录 401，业务失败 200 envelope，错误不泄漏 SQL 或 Cookie。

前端测试：

- 个人中心绑定流程状态展示。
- 二维码来源切换和预览。
- 任务列表继续按分组展示。
- 手动状态显示切换按钮，Live 状态显示刷新按钮。
- 离线模式下 Live 刷新禁用，手动状态仍可入队。

## 实施顺序建议

1. 重构站内数据模型：二维码来源、完成状态枚举、任务外部字段。
2. 抽取二维码读取逻辑，统一支持上传图片和生成图片。
3. 添加 B 站账号绑定后端适配层和个人中心 UI。
4. 添加任务同步模块和任务表同步逻辑。
5. 添加 Live 状态刷新接口和前端状态按钮切换。
6. 更新离线快照与同步队列。
7. 更新 README、设计规格和发布说明。

## 参考资料

- `docs/bws2026-live-api.md`：从官方 BWS 2026 页面脚本整理的接口信息。
- B 站 Web 扫码登录社区整理资料：`https://github.com/pskdje/bilibili-API-collect/blob/main/docs/login/login_action/QR.md`。

## 自检结果

- 没有把 B 站 Cookie 暴露给前端。
- 没有新增 RESTful 路径或非 GET / POST 业务接口。
- 没有引入对象存储、Redis 或复杂部署。
- 任务同步失败不会阻塞系统使用。
- Live 状态和手动状态有清晰的数据模型与前端交互边界。
- 离线能力保留，并明确 Live 状态离线只能读快照。
