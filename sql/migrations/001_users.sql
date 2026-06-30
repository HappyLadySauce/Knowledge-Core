CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    email TEXT UNIQUE,
    avatar TEXT NOT NULL DEFAULT '',
    bio TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'user')),
    status TEXT NOT NULL CHECK (status IN ('active', 'disabled')),
    token_version INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_users_role_status
ON users (role, status);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    revoked_at TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id
ON refresh_tokens (user_id);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at
ON refresh_tokens (expires_at);

CREATE TABLE IF NOT EXISTS login_attempts (
    user_id INTEGER PRIMARY KEY,
    failed_count INTEGER NOT NULL DEFAULT 0,
    last_failed_at TEXT,
    locked_until TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
