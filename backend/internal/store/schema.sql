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
  day TEXT NOT NULL CHECK (day IN ('20260710', '20260711', '20260712', 'friday', 'saturday', 'sunday')),
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
