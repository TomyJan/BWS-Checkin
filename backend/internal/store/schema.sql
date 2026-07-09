CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  oidc_subject TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL,
  avatar_url TEXT NOT NULL DEFAULT '',
  qr_image_path TEXT NOT NULL DEFAULT '',
  qr_source TEXT NOT NULL DEFAULT 'uploaded' CHECK (qr_source IN ('uploaded', 'bilibili_generated')),
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS bilibili_accounts (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  mid TEXT NOT NULL,
  uname TEXT NOT NULL,
  face_url TEXT NOT NULL DEFAULT '',
  cookie_ciphertext TEXT NOT NULL,
  cookie_expires_at TEXT NOT NULL DEFAULT '',
  refresh_token_ciphertext TEXT NOT NULL DEFAULT '',
  last_validated_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS groups (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  day TEXT NOT NULL CHECK (day IN ('friday', 'saturday', 'sunday')),
  description TEXT NOT NULL DEFAULT '',
  owner_user_id TEXT NOT NULL REFERENCES users(id),
  join_locked INTEGER NOT NULL DEFAULT 0,
  archived_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS group_members (
  group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role TEXT NOT NULL CHECK (role IN ('owner', 'member')),
  joined_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (group_id, user_id)
);

CREATE TABLE IF NOT EXISTS tasks (
  id TEXT PRIMARY KEY,
  external_id TEXT NOT NULL DEFAULT '',
  group_name TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  reward_coins INTEGER NOT NULL DEFAULT 0,
  description TEXT NOT NULL DEFAULT '',
  image_url TEXT NOT NULL DEFAULT '',
  venue_id TEXT NOT NULL DEFAULT '',
  venue_name TEXT NOT NULL DEFAULT '',
  event_day TEXT NOT NULL DEFAULT '',
  sync_source TEXT NOT NULL DEFAULT 'default' CHECK (sync_source IN ('default', 'bilibili')),
  sort_order INTEGER NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS task_completions (
  group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  task_id TEXT NOT NULL REFERENCES tasks(id),
  target_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  checked_by_user_id TEXT NOT NULL REFERENCES users(id),
  completed INTEGER NOT NULL DEFAULT 1,
  status TEXT NOT NULL DEFAULT 'manual_completed' CHECK (status IN ('manual_incomplete', 'manual_completed', 'live_incomplete', 'live_completed')),
  source TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'live')),
  completed_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  live_checked_at TEXT NOT NULL DEFAULT '',
  live_stale INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (group_id, task_id, target_user_id)
);

CREATE TABLE IF NOT EXISTS task_sync_state (
  id TEXT PRIMARY KEY,
  last_success_at TEXT NOT NULL DEFAULT '',
  last_error_at TEXT NOT NULL DEFAULT '',
  last_error_code TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS audit_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  actor_user_id TEXT NOT NULL REFERENCES users(id),
  action TEXT NOT NULL,
  group_id TEXT NOT NULL DEFAULT '',
  target_user_id TEXT,
  task_id TEXT NOT NULL DEFAULT '',
  metadata TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO tasks (id, group_name, name, title, reward_coins, description, sort_order, enabled) VALUES
  ('rainbow-station', '场馆打卡', '彩虹补给站', '完成彩虹补给站互动', 3, '在彩虹补给站完成互动并出示二维码。', 10, 1),
  ('stage-support', '舞台任务', '舞台应援任务', '完成主舞台应援', 5, '在主舞台完成应援任务并领取奖励。', 20, 1),
  ('stamp-rally', '场馆打卡', '乐园集章点', '完成乐园集章点打卡', 2, '到达集章点完成盖章或扫码确认。', 30, 1),
  ('photo-spot', '互动任务', '主题合影点', '完成主题合影点互动', 2, '在主题合影点完成互动拍照后出示二维码。', 40, 1);
