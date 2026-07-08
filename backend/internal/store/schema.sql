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
  join_locked INTEGER NOT NULL DEFAULT 0,
  archived_at TEXT NOT NULL DEFAULT '',
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
  completed INTEGER NOT NULL DEFAULT 1,
  completed_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (group_id, task_id, target_user_id)
);

INSERT OR IGNORE INTO tasks (id, name, sort_order, enabled) VALUES
  ('rainbow-station', '彩虹补给站', 10, 1),
  ('stage-support', '舞台应援任务', 20, 1),
  ('stamp-rally', '乐园集章点', 30, 1),
  ('photo-spot', '主题合影点', 40, 1);
