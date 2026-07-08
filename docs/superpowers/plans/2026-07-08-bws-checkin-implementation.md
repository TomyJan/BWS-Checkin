# BWS Checkin MVP 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 构建 BWS Checkin 的可运行 MVP：OIDC/mock 登录、二维码上传、互助组、点位任务、成员打卡状态和符合设计的前端交互。

**架构：** 前后端分离。后端使用 Go + chi 提供 `/api/v1` JSON API、Session Cookie、SQLite 数据持久化和本地二维码文件存储；前端使用 Vite + React + TypeScript + MUI 通过 API 驱动页面状态。MVP 优先保证本地开发可跑、核心行为完整，生产 OIDC 接口保留配置入口。

**技术栈：** Go 1.25、chi、SQLite、Vite、React、TypeScript、MUI Material、TanStack Query、React Router。

---

## 文件结构

### 后端

- 创建：`backend/go.mod`，Go 模块定义。
- 创建：`backend/cmd/server/main.go`，HTTP 服务入口。
- 创建：`backend/internal/app/app.go`，组装配置、数据库、路由和静态文件服务。
- 创建：`backend/internal/config/config.go`，读取端口、数据库路径、上传目录、开发登录开关和 OIDC 配置。
- 创建：`backend/internal/http/router.go`，注册 `/api/v1`、`/auth/oidc` 和健康检查路由。
- 创建：`backend/internal/http/session.go`，Session Cookie 读写和鉴权中间件。
- 创建：`backend/internal/http/handlers.go`，API handler。
- 创建：`backend/internal/store/store.go`，SQLite 访问层。
- 创建：`backend/internal/store/schema.sql`，数据库表结构和固定点位 seed。
- 创建：`backend/internal/domain/types.go`，API 请求和响应类型。
- 创建：`backend/internal/filestore/local.go`，二维码文件保存和删除。
- 测试：`backend/internal/store/store_test.go`，数据库约束和状态记录测试。
- 测试：`backend/internal/http/handlers_test.go`，API 行为测试。

### 前端

- 创建：`frontend/package.json`，前端依赖和脚本。
- 创建：`frontend/index.html`，Vite 入口。
- 创建：`frontend/src/main.tsx`，React 根入口。
- 创建：`frontend/src/App.tsx`，路由和全局 providers。
- 创建：`frontend/src/theme.ts`，MUI 主题和深色模式。
- 创建：`frontend/src/api/client.ts`，fetch 封装。
- 创建：`frontend/src/api/types.ts`，前后端共享响应类型的前端定义。
- 创建：`frontend/src/features/auth/AuthGate.tsx`，登录态守卫。
- 创建：`frontend/src/features/home/HomePage.tsx`，我的互助组首页。
- 创建：`frontend/src/features/groups/GroupPage.tsx`，全屏二维码和任务浮层。
- 创建：`frontend/src/features/groups/GroupDialogs.tsx`，创建组、加入组、点位选择弹窗。
- 创建：`frontend/src/features/profile/QRCodeUpload.tsx`，二维码上传入口。
- 创建：`frontend/src/styles.css`，全局布局、全屏二维码视图和浮层动画样式。

### 项目根目录

- 修改：`README.md`，记录本地启动方式。
- 修改：`.gitignore`，忽略数据库、上传图片、依赖和构建产物。

---

## 任务 1：后端项目骨架

**文件：**
- 创建：`backend/go.mod`
- 创建：`backend/cmd/server/main.go`
- 创建：`backend/internal/config/config.go`
- 创建：`backend/internal/app/app.go`
- 创建：`backend/internal/http/router.go`

- [ ] **步骤 1：创建 Go 模块和最小服务入口**

`backend/go.mod`：

```go
module bws-checkin/backend

go 1.25

require (
	github.com/go-chi/chi/v5 v5.3.1
	modernc.org/sqlite v1.53.0
)
```

`backend/cmd/server/main.go`：

```go
package main

import (
	"log"
	"net/http"

	"bws-checkin/backend/internal/app"
	"bws-checkin/backend/internal/config"
)

func main() {
	cfg := config.Load()
	handler, cleanup, err := app.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	log.Printf("listening on %s", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, handler))
}
```

- [ ] **步骤 2：实现配置读取**

`backend/internal/config/config.go`：

```go
package config

import "os"

type Config struct {
	Addr       string
	DBPath     string
	UploadDir  string
	DevAuth    bool
	PublicBase string
}

func Load() Config {
	return Config{
		Addr:       env("BWS_ADDR", ":8080"),
		DBPath:     env("BWS_DB", "data/bws.db"),
		UploadDir:  env("BWS_UPLOAD_DIR", "data/uploads"),
		DevAuth:    env("BWS_DEV_AUTH", "1") == "1",
		PublicBase: env("BWS_PUBLIC_BASE", "http://localhost:5173"),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
```

- [ ] **步骤 3：实现健康检查路由**

`backend/internal/http/router.go`：

```go
package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return r
}
```

`backend/internal/app/app.go`：

```go
package app

import (
	"net/http"

	"bws-checkin/backend/internal/config"
	httpapi "bws-checkin/backend/internal/http"
)

func New(cfg config.Config) (http.Handler, func(), error) {
	return httpapi.NewRouter(), func() {}, nil
}
```

- [ ] **步骤 4：运行后端检查**

运行：

```bash
cd backend
go mod tidy
go test ./...
```

预期：`go test ./...` 退出码为 0。

- [ ] **步骤 5：Commit**

```bash
git add backend
git commit -m "feat(后端): 添加 Go 服务骨架"
```

---

## 任务 2：SQLite 表结构和存储层

**文件：**
- 创建：`backend/internal/domain/types.go`
- 创建：`backend/internal/store/schema.sql`
- 创建：`backend/internal/store/store.go`
- 创建：`backend/internal/store/store_test.go`

- [ ] **步骤 1：编写存储层失败测试**

`backend/internal/store/store_test.go`：

```go
package store

import (
	"testing"
)

func TestCreateGroupRejectsDuplicateID(t *testing.T) {
	s := newTestStore(t)
	user := mustCreateUser(t, s, "oidc-owner", "Owner")

	if err := s.CreateGroup(t.Context(), CreateGroupInput{
		ID: "bw2026-fri", Name: "BW2026 周五", Day: "friday", OwnerUserID: user.ID,
	}); err != nil {
		t.Fatalf("create group: %v", err)
	}

	err := s.CreateGroup(t.Context(), CreateGroupInput{
		ID: "bw2026-fri", Name: "重复组", Day: "friday", OwnerUserID: user.ID,
	})
	if err == nil {
		t.Fatal("expected duplicate group ID to fail")
	}
}

func TestCompleteTaskIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	owner := mustCreateUser(t, s, "oidc-owner", "Owner")
	member := mustCreateUser(t, s, "oidc-member", "Member")
	mustCreateGroup(t, s, "bw2026-fri", owner.ID)
	if err := s.JoinGroup(t.Context(), "bw2026-fri", member.ID); err != nil {
		t.Fatalf("join group: %v", err)
	}

	if err := s.MarkComplete(t.Context(), "bw2026-fri", "rainbow-station", member.ID, owner.ID); err != nil {
		t.Fatalf("mark complete: %v", err)
	}
	if err := s.MarkComplete(t.Context(), "bw2026-fri", "rainbow-station", member.ID, owner.ID); err != nil {
		t.Fatalf("repeat mark complete: %v", err)
	}

	tasks, err := s.GroupTasks(t.Context(), "bw2026-fri")
	if err != nil {
		t.Fatalf("group tasks: %v", err)
	}
	if tasks[0].CompletedCount != 1 {
		t.Fatalf("completed count = %d, want 1", tasks[0].CompletedCount)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：

```bash
cd backend
go test ./internal/store
```

预期：FAIL，报错包含 `undefined: Store` 或相关未定义类型。

- [ ] **步骤 3：实现 domain 类型和 schema**

`backend/internal/domain/types.go`：

```go
package domain

import "time"

type User struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl"`
	QRImageURL  string `json:"qrImageUrl"`
}

type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Day         string `json:"day"`
	Description string `json:"description"`
	Role        string `json:"role"`
	MemberCount int    `json:"memberCount"`
	TaskCount   int    `json:"taskCount"`
}

type Member struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"displayName"`
	QRImageURL  string `json:"qrImageUrl"`
}

type TaskStatus struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	SortOrder      int                `json:"sortOrder"`
	CompletedCount int                `json:"completedCount"`
	TotalCount     int                `json:"totalCount"`
	Members        []MemberCompletion `json:"members"`
}

type MemberCompletion struct {
	Member       Member     `json:"member"`
	Completed    bool       `json:"completed"`
	CompletedAt  *time.Time `json:"completedAt"`
	CheckedByID  *int64     `json:"checkedById"`
	CheckedByName string    `json:"checkedByName"`
}
```

`backend/internal/store/schema.sql`：

```sql
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  oidc_subject TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL,
  avatar_url TEXT NOT NULL DEFAULT '',
  qr_image_path TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS groups (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  day TEXT NOT NULL CHECK (day IN ('friday', 'saturday', 'sunday')),
  description TEXT NOT NULL DEFAULT '',
  owner_user_id INTEGER NOT NULL REFERENCES users(id),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS group_members (
  group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role TEXT NOT NULL CHECK (role IN ('owner', 'member')),
  joined_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (group_id, user_id)
);

CREATE TABLE IF NOT EXISTS tasks (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  sort_order INTEGER NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS task_completions (
  group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  task_id TEXT NOT NULL REFERENCES tasks(id),
  target_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  checked_by_user_id INTEGER NOT NULL REFERENCES users(id),
  completed_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (group_id, task_id, target_user_id)
);

INSERT OR IGNORE INTO tasks (id, name, sort_order, enabled) VALUES
  ('rainbow-station', '彩虹补给站', 10, 1),
  ('stage-support', '舞台应援任务', 20, 1),
  ('stamp-rally', '乐园集章点', 30, 1),
  ('photo-spot', '主题合影点', 40, 1);
```

- [ ] **步骤 4：实现最小存储层**

`backend/internal/store/store.go`：

```go
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"os"
	"path/filepath"

	"bws-checkin/backend/internal/domain"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

type Store struct {
	db *sql.DB
}

type CreateGroupInput struct {
	ID          string
	Name        string
	Day         string
	Description string
	OwnerUserID int64
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func OpenMemory() (*Store, error) {
	db, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	body, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	_, err = s.db.Exec(string(body))
	return err
}

func (s *Store) UpsertUser(ctx context.Context, subject, displayName string) (domain.User, error) {
	if subject == "" || displayName == "" {
		return domain.User{}, errors.New("subject and display name are required")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (oidc_subject, display_name)
		VALUES (?, ?)
		ON CONFLICT(oidc_subject) DO UPDATE SET display_name = excluded.display_name, updated_at = CURRENT_TIMESTAMP
	`, subject, displayName)
	if err != nil {
		return domain.User{}, err
	}
	return s.UserBySubject(ctx, subject)
}
```

同文件必须实现以下方法：

```go
func (s *Store) UserBySubject(ctx context.Context, subject string) (domain.User, error)
func (s *Store) UserByID(ctx context.Context, id int64) (domain.User, error)
func (s *Store) UpdateUserQR(ctx context.Context, userID int64, path string) error
func (s *Store) CreateGroup(ctx context.Context, input CreateGroupInput) error
func (s *Store) JoinGroup(ctx context.Context, groupID string, userID int64) error
func (s *Store) UserGroups(ctx context.Context, userID int64) ([]domain.Group, error)
func (s *Store) GroupTasks(ctx context.Context, groupID string) ([]domain.TaskStatus, error)
func (s *Store) MarkComplete(ctx context.Context, groupID string, taskID string, targetUserID int64, checkedByUserID int64) error
```

每个方法只封装一个明确的数据库操作；`GroupTasks` 负责组装当前组成员在每个点位下的完成状态。

- [ ] **步骤 5：补齐测试 helper**

在 `store_test.go` 增加：

```go
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenMemory()
	if err != nil {
		t.Fatalf("open memory store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func mustCreateUser(t *testing.T, s *Store, subject, name string) domain.User {
	t.Helper()
	user, err := s.UpsertUser(t.Context(), subject, name)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func mustCreateGroup(t *testing.T, s *Store, id string, ownerID int64) {
	t.Helper()
	if err := s.CreateGroup(t.Context(), CreateGroupInput{
		ID: id, Name: "BW2026 周五", Day: "friday", OwnerUserID: ownerID,
	}); err != nil {
		t.Fatalf("create group: %v", err)
	}
}
```

- [ ] **步骤 6：运行测试验证通过**

运行：

```bash
cd backend
go test ./internal/store
```

预期：PASS。

- [ ] **步骤 7：Commit**

```bash
git add backend/internal/domain backend/internal/store
git commit -m "feat(后端): 添加 SQLite 存储层"
```

---

## 任务 3：Session、mock 登录和 `/api/v1/me`

**文件：**
- 创建：`backend/internal/http/session.go`
- 修改：`backend/internal/http/router.go`
- 创建：`backend/internal/http/handlers.go`
- 创建：`backend/internal/http/handlers_test.go`
- 修改：`backend/internal/app/app.go`

- [ ] **步骤 1：编写 API 失败测试**

`backend/internal/http/handlers_test.go`：

```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"bws-checkin/backend/internal/store"
)

func TestDevLoginAndMe(t *testing.T) {
	s, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	h := NewRouter(Deps{Store: s, DevAuth: true})

	login := httptest.NewRequest(http.MethodPost, "/api/v1/dev/login?name=TomyJan", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, login)
	if w.Code != http.StatusOK {
		t.Fatalf("login status = %d", w.Code)
	}

	me := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	for _, c := range w.Result().Cookies() {
		me.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, me)
	if w.Code != http.StatusOK {
		t.Fatalf("me status = %d", w.Code)
	}
}

func TestMeRequiresLogin(t *testing.T) {
	s, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	h := NewRouter(Deps{Store: s, DevAuth: true})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：

```bash
cd backend
go test ./internal/http
```

预期：FAIL，报错包含 `undefined: Deps` 或 `too many arguments in call to NewRouter`。

- [ ] **步骤 3：实现 Session Cookie**

`backend/internal/http/session.go`：

```go
package httpapi

import (
	"net/http"
	"strconv"
)

const sessionCookieName = "bws_session"

func setSession(w http.ResponseWriter, userID int64) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    strconv.FormatInt(userID, 10),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func sessionUserID(r *http.Request) (int64, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return 0, false
	}
	id, err := strconv.ParseInt(cookie.Value, 10, 64)
	return id, err == nil
}
```

- [ ] **步骤 4：实现 handlers 和 `/api/v1` 路由**

`backend/internal/http/handlers.go`：

```go
package httpapi

import (
	"encoding/json"
	"net/http"

	"bws-checkin/backend/internal/store"
)

type Deps struct {
	Store   *store.Store
	DevAuth bool
}

type Handler struct {
	deps Deps
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (h Handler) devLogin(w http.ResponseWriter, r *http.Request) {
	if !h.deps.DevAuth {
		http.NotFound(w, r)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "TomyJan"
	}
	user, err := h.deps.Store.UpsertUser(r.Context(), "dev:"+name, name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	setSession(w, user.ID)
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}
```

修改 `router.go`，让 `NewRouter(deps Deps)` 注册：

```go
r.Route("/api/v1", func(r chi.Router) {
	h := Handler{deps: deps}
	r.Post("/dev/login", h.devLogin)
	r.Get("/me", h.me)
})
```

- [ ] **步骤 5：运行测试验证通过**

运行：

```bash
cd backend
go test ./internal/http
```

预期：PASS。

- [ ] **步骤 6：Commit**

```bash
git add backend/internal/http backend/internal/app
git commit -m "feat(后端): 添加 mock 登录和用户接口"
```

---

## 任务 4：互助组和任务 API

**文件：**
- 修改：`backend/internal/http/handlers.go`
- 修改：`backend/internal/http/router.go`
- 修改：`backend/internal/store/store.go`
- 修改：`backend/internal/http/handlers_test.go`

- [ ] **步骤 1：编写 API 行为测试**

在 `handlers_test.go` 增加：

```go
func TestGroupsAndTasksFlow(t *testing.T) {
	s, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	h := NewRouter(Deps{Store: s, DevAuth: true})
	cookies := loginForTest(t, h, "TomyJan")

	req := jsonRequest(t, http.MethodPost, "/api/v1/groups", map[string]any{
		"id": "bw2026-fri", "name": "BW2026 周五", "day": "friday", "description": "测试组",
	})
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create group status = %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/groups/bw2026-fri/tasks", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("tasks status = %d", w.Code)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：

```bash
cd backend
go test ./internal/http
```

预期：FAIL，接口返回 404。

- [ ] **步骤 3：实现 API**

在 `/api/v1` 下注册：

```go
r.Get("/groups", h.listGroups)
r.Post("/groups", h.createGroup)
r.Get("/groups/{groupId}", h.groupDetail)
r.Post("/groups/{groupId}/join", h.joinGroup)
r.Delete("/groups/{groupId}/members/{userId}", h.removeMember)
r.Get("/groups/{groupId}/tasks", h.groupTasks)
r.Post("/groups/{groupId}/tasks/{taskId}/members/{userId}/complete", h.completeTask)
r.Delete("/groups/{groupId}/tasks/{taskId}/members/{userId}/complete", h.uncompleteTask)
```

handler 行为：

- 所有组和任务接口必须登录。
- 创建组时当前用户自动成为 `owner`。
- 加入组时当前用户成为 `member`。
- 移除成员只允许 `owner`。
- 标记完成的 `checked_by_user_id` 使用当前登录用户。
- 重复标记完成返回 200，不新增记录。

- [ ] **步骤 4：运行测试验证通过**

运行：

```bash
cd backend
go test ./internal/http ./internal/store
```

预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add backend/internal/http backend/internal/store
git commit -m "feat(后端): 添加互助组和任务接口"
```

---

## 任务 5：二维码上传和静态文件访问

**文件：**
- 创建：`backend/internal/filestore/local.go`
- 修改：`backend/internal/http/handlers.go`
- 修改：`backend/internal/http/router.go`
- 修改：`backend/internal/store/store.go`
- 修改：`backend/internal/app/app.go`

- [ ] **步骤 1：编写上传失败测试**

在 `handlers_test.go` 增加 multipart 上传测试，断言：

- 未登录上传返回 401。
- 登录后上传 PNG 返回 200。
- 再次 `GET /api/v1/me` 时 `qrImageUrl` 不为空。

- [ ] **步骤 2：运行测试验证失败**

运行：

```bash
cd backend
go test ./internal/http
```

预期：FAIL，上传接口 404。

- [ ] **步骤 3：实现本地文件存储**

`backend/internal/filestore/local.go`：

```go
package filestore

import (
	"io"
	"os"
	"path/filepath"
)

type Local struct {
	Dir string
}

func (l Local) SaveQR(userID int64, ext string, src io.Reader) (string, error) {
	if err := os.MkdirAll(l.Dir, 0755); err != nil {
		return "", err
	}
	name := filepath.Join(l.Dir, strconv.FormatInt(userID, 10)+ext)
	dst, err := os.Create(name)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return name, err
}
```

完整实现需要包含 `strconv` import，并把返回路径转换为 `/uploads/<file>` URL。

- [ ] **步骤 4：注册上传和静态文件路由**

新增：

```go
r.Post("/api/v1/me/qr", h.uploadQR)
r.Delete("/api/v1/me/qr", h.deleteQR)
r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))
```

上传限制：

- 最大 5 MB。
- 允许 `.png`、`.jpg`、`.jpeg`、`.webp`。
- 保存后更新 `users.qr_image_path`。

- [ ] **步骤 5：运行测试验证通过**

运行：

```bash
cd backend
go test ./...
```

预期：PASS。

- [ ] **步骤 6：Commit**

```bash
git add backend
git commit -m "feat(后端): 添加二维码上传"
```

---

## 任务 6：前端项目骨架和 API 客户端

**文件：**
- 创建：`frontend/package.json`
- 创建：`frontend/index.html`
- 创建：`frontend/src/main.tsx`
- 创建：`frontend/src/App.tsx`
- 创建：`frontend/src/theme.ts`
- 创建：`frontend/src/api/client.ts`
- 创建：`frontend/src/api/types.ts`
- 创建：`frontend/src/styles.css`

- [ ] **步骤 1：创建 Vite React 项目文件**

`frontend/package.json`：

```json
{
  "scripts": {
    "dev": "vite --host 127.0.0.1",
    "build": "tsc -b && vite build",
    "preview": "vite preview --host 127.0.0.1"
  },
  "dependencies": {
    "@emotion/react": "^11.14.0",
    "@emotion/styled": "^11.14.1",
    "@mui/icons-material": "^9.2.0",
    "@mui/material": "^9.2.0",
    "@tanstack/react-query": "^5.101.2",
    "vite": "^8.1.3",
    "react": "^19.2.7",
    "react-dom": "^19.2.7",
    "react-router-dom": "^7.18.1"
  },
  "devDependencies": {
    "@types/react": "^19.2.17",
    "@types/react-dom": "^19.2.3",
    "typescript": "^6.0.3",
    "@vitejs/plugin-react": "^6.0.3"
  }
}
```

- [ ] **步骤 2：实现 API 客户端**

`frontend/src/api/client.ts`：

```ts
const API_BASE = "/api/v1";

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    credentials: "include",
    headers: init?.body instanceof FormData ? init.headers : { "Content-Type": "application/json", ...init?.headers },
    ...init,
  });

  if (!response.ok) {
    throw new Error(`API ${response.status}`);
  }

  return response.json() as Promise<T>;
}
```

- [ ] **步骤 3：实现主题和根入口**

`frontend/src/theme.ts` 定义 `createTheme`，设置：

- `shape.borderRadius = 22`
- Button、Dialog、Card 使用大圆角
- `colorSchemes` 或根据 `prefers-color-scheme` 切换浅深色

`frontend/src/App.tsx` 设置 `QueryClientProvider`、`ThemeProvider`，并注册 `/` 和 `/groups/:groupId` 两个路由；在对应页面实现前，路由组件返回页面标题文本。

- [ ] **步骤 4：运行构建验证**

运行：

```bash
cd frontend
npm install
npm run build
```

预期：构建成功。

- [ ] **步骤 5：Commit**

```bash
git add frontend
git commit -m "feat(前端): 添加 React 项目骨架"
```

---

## 任务 7：首页、登录守卫和二维码提示

**文件：**
- 创建：`frontend/src/features/auth/AuthGate.tsx`
- 创建：`frontend/src/features/home/HomePage.tsx`
- 创建：`frontend/src/features/profile/QRCodeUpload.tsx`
- 修改：`frontend/src/App.tsx`
- 修改：`frontend/src/api/types.ts`

- [ ] **步骤 1：定义前端类型**

`frontend/src/api/types.ts`：

```ts
export interface User {
  id: number;
  displayName: string;
  avatarUrl: string;
  qrImageUrl: string;
}

export interface Group {
  id: string;
  name: string;
  day: "friday" | "saturday" | "sunday";
  description: string;
  role: "owner" | "member";
  memberCount: number;
  taskCount: number;
}
```

- [ ] **步骤 2：实现登录守卫**

`AuthGate.tsx`：

```tsx
import { Button, CircularProgress, Stack, Typography } from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import { api } from "../../api/client";
import type { User } from "../../api/types";

export function AuthGate({ children }: { children: React.ReactNode }) {
  const me = useQuery({ queryKey: ["me"], queryFn: () => api<{ user: User }>("/me") });

  if (me.isLoading) return <CircularProgress />;
  if (me.isError) {
    return (
      <Stack minHeight="100vh" alignItems="center" justifyContent="center" spacing={2}>
        <Typography variant="h4">BWS Checkin</Typography>
        <Button variant="contained" href="/api/v1/dev/login">登录</Button>
      </Stack>
    );
  }
  return children;
}
```

- [ ] **步骤 3：实现首页**

`HomePage.tsx` 需要满足：

- 标题「我的互助组」和副标题在左侧。
- 右侧 `Add` 图标按钮。
- 点击后打开 MUI `Menu`，菜单项为「创建互助组」和「加入互助组」。
- 仅当 `me.user.qrImageUrl` 为空时显示上传提示。
- 互助组列表从 `GET /api/v1/groups` 获取。

- [ ] **步骤 4：运行前端构建**

运行：

```bash
cd frontend
npm run build
```

预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add frontend/src
git commit -m "feat(前端): 添加首页和登录守卫"
```

---

## 任务 8：创建、加入互助组弹窗

**文件：**
- 创建：`frontend/src/features/groups/GroupDialogs.tsx`
- 修改：`frontend/src/features/home/HomePage.tsx`

- [ ] **步骤 1：实现创建组弹窗**

`GroupDialogs.tsx` 导出 `CreateGroupDialog`：

```tsx
export interface CreateGroupValues {
  id: string;
  name: string;
  day: "friday" | "saturday" | "sunday";
  description: string;
}
```

UI 要求：

- `TextField`：名称。
- `TextField`：ID。
- `ToggleButtonGroup` 或 `Tabs`：周五、周六、周日。
- `TextField multiline`：说明内容。
- 提交调用 `POST /api/v1/groups`。

- [ ] **步骤 2：实现加入组弹窗**

`JoinGroupDialog`：

- 输入组 ID。
- 支持读取 URL query 中的 `invite` 作为默认值。
- 提交调用 `POST /api/v1/groups/{groupId}/join`。

- [ ] **步骤 3：接入首页菜单**

在 `HomePage.tsx` 中：

- 点击「创建互助组」打开 `CreateGroupDialog`。
- 点击「加入互助组」打开 `JoinGroupDialog`。
- 成功后刷新 `["groups"]` query。

- [ ] **步骤 4：运行构建**

运行：

```bash
cd frontend
npm run build
```

预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add frontend/src/features
git commit -m "feat(前端): 添加互助组创建和加入弹窗"
```

---

## 任务 9：互助组详情页全屏二维码和任务浮层

**文件：**
- 创建：`frontend/src/features/groups/GroupPage.tsx`
- 修改：`frontend/src/App.tsx`
- 修改：`frontend/src/api/types.ts`
- 修改：`frontend/src/styles.css`

- [ ] **步骤 1：定义任务状态类型**

`api/types.ts` 增加：

```ts
export interface Member {
  id: number;
  displayName: string;
  qrImageUrl: string;
}

export interface MemberCompletion {
  member: Member;
  completed: boolean;
  completedAt: string | null;
  checkedById: number | null;
  checkedByName: string;
}

export interface TaskStatus {
  id: string;
  name: string;
  sortOrder: number;
  completedCount: number;
  totalCount: number;
  members: MemberCompletion[];
}
```

- [ ] **步骤 2：实现详情页布局**

`GroupPage.tsx` 行为：

- 读取 `GET /api/v1/groups/{groupId}` 和 `GET /api/v1/groups/{groupId}/tasks`。
- 默认选中第一个任务。
- 默认选中第一个成员。
- 二维码图片使用 `object-fit: contain`，外层全屏黑色背景。
- 点击主区域切换 `uiVisible`。
- UI 可见时显示顶部和底部浮层；隐藏时加 CSS class 触发滑出动画。

- [ ] **步骤 3：实现成员切换和完成按钮**

详情页行为：

- 左右按钮或键盘方向键切换成员。
- 当前成员状态高亮。
- 点击「标记完成」调用 `POST /api/v1/groups/{groupId}/tasks/{taskId}/members/{userId}/complete`。
- 成功后刷新任务 query。
- 已完成成员显示完成时间和打卡人。

- [ ] **步骤 4：实现点位列表弹窗**

点击点位名称打开 MUI `Dialog` 或 `BottomSheet` 风格 `Drawer`：

- 列出所有点位名称。
- 显示每个点位的 `completedCount/totalCount`。
- 点击点位后切换当前任务并关闭弹窗。

- [ ] **步骤 5：运行构建**

运行：

```bash
cd frontend
npm run build
```

预期：PASS。

- [ ] **步骤 6：Commit**

```bash
git add frontend/src
git commit -m "feat(前端): 添加互助组打卡视图"
```

---

## 任务 10：本地联调、README 和最终验证

**文件：**
- 修改：`README.md`
- 修改：`.gitignore`
- 创建：`scripts/dev.ps1`

- [ ] **步骤 1：完善忽略规则**

`.gitignore` 增加：

```gitignore
backend/data/
frontend/node_modules/
frontend/dist/
```

- [ ] **步骤 2：更新 README**

`README.md` 增加：

```markdown
## 本地开发

### 后端

```bash
cd backend
go run ./cmd/server
```

### 前端

```bash
cd frontend
npm install
npm run dev
```

开发登录地址：`/api/v1/dev/login`。生产环境应关闭 `BWS_DEV_AUTH` 并配置 OIDC。
```

- [ ] **步骤 3：运行完整后端测试**

运行：

```bash
cd backend
go test ./...
```

预期：PASS。

- [ ] **步骤 4：运行前端构建**

运行：

```bash
cd frontend
npm run build
```

预期：PASS。

- [ ] **步骤 5：启动本地服务验证**

终端 1：

```bash
cd backend
go run ./cmd/server
```

终端 2：

```bash
cd frontend
npm run dev
```

手动验证：

- 打开前端首页。
- 点击登录进入 mock 登录。
- 未上传二维码时首页显示上传提示。
- 创建 `BW2026 周五` 互助组。
- 进入互助组详情页。
- 打开点位列表并切换点位。
- 标记某个成员完成后，进度从 `0/N` 更新为 `1/N`。

- [ ] **步骤 6：Commit**

```bash
git add README.md .gitignore scripts frontend backend
git commit -m "docs(开发): 更新本地运行说明"
```

---

## 自检结果

- 规格覆盖：登录、二维码上传、首页、互助组创建 / 加入、创建者管理、全屏二维码、点位列表、完成状态、`/api/v1` API 前缀均有对应任务。
- TDD 覆盖：后端存储层、登录 API、互助组 API、任务 API、二维码上传 API 按失败测试到实现的顺序推进；前端每个页面任务都通过构建验证，最终用手动联调覆盖交互行为。
- 范围控制：MVP 不实现真实线上任务同步，不实现生产 OIDC 配置 UI，不实现细粒度权限系统。
