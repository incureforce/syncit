CREATE TABLE IF NOT EXISTS metadata (
	name TEXT PRIMARY KEY,
	data TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS client (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	tags TEXT NOT NULL DEFAULT '[]',
	synced_at DATETIME,
	created_at TEXT NOT NULL,
	deleted_at TEXT,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS mount (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	root_path TEXT NOT NULL,
	created_at TEXT NOT NULL,
	deleted_at TEXT,
	updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mount_name ON mount(name) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS mount_file (
	id TEXT PRIMARY KEY,
	mount_id TEXT NOT NULL,
	local_file_path TEXT NOT NULL,
	local_file_hash TEXT NOT NULL DEFAULT '',
	local_file_tags TEXT NOT NULL DEFAULT '[]',
	local_file_version INTEGER NOT NULL DEFAULT 0,
	remote_file_hash TEXT NOT NULL DEFAULT '',
	remote_file_tags TEXT NOT NULL DEFAULT '[]',
	remote_file_version INTEGER NOT NULL DEFAULT 0,
	blob_key TEXT NOT NULL DEFAULT '',
	file_size INTEGER NOT NULL DEFAULT 0,
	conflict INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	deleted_at TEXT,
	updated_at TEXT NOT NULL,
	FOREIGN KEY (mount_id) REFERENCES mount(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mount_file_unique ON mount_file(mount_id, local_file_path) WHERE deleted_at IS NULL;

PRAGMA user_version = 1;
