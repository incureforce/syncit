CREATE TABLE IF NOT EXISTS metadata (
	name TEXT PRIMARY KEY,
	data TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS clients (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	tags TEXT NOT NULL DEFAULT '[]',
	synced_at DATETIME,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	deleted_at DATETIME
);

-- Global mount names (not per-client). Files and versions live under a mount.
CREATE TABLE IF NOT EXISTS mounts (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_mounts_name ON mounts(name);

-- Which clients use which global mounts (for pull visibility and future sync).
CREATE TABLE IF NOT EXISTS client_mounts (
	id TEXT PRIMARY KEY,
	client_id TEXT NOT NULL,
	mount_id TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	deleted_at DATETIME,
	UNIQUE(client_id, mount_id),
	FOREIGN KEY (client_id) REFERENCES clients(id),
	FOREIGN KEY (mount_id) REFERENCES mounts(id)
);

CREATE INDEX IF NOT EXISTS idx_client_mounts_client ON client_mounts(client_id);
CREATE INDEX IF NOT EXISTS idx_client_mounts_mount ON client_mounts(mount_id);

CREATE TABLE IF NOT EXISTS mount_files (
	id TEXT PRIMARY KEY,
	mount_id TEXT NOT NULL,
	path TEXT NOT NULL,
	tags TEXT NOT NULL DEFAULT '[]',
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	deleted_at DATETIME,
	UNIQUE(mount_id, path),
	FOREIGN KEY (mount_id) REFERENCES mounts(id)
);

CREATE INDEX IF NOT EXISTS idx_mount_files_mount ON mount_files(mount_id);

CREATE TABLE IF NOT EXISTS mount_file_versions (
	id TEXT PRIMARY KEY,
	mount_file_id TEXT NOT NULL,
	file_version INTEGER NOT NULL,
	blob_key TEXT NOT NULL,
	file_size INTEGER NOT NULL,
	file_hash TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	deleted_at DATETIME,
	UNIQUE(mount_file_id, file_version),
	FOREIGN KEY (mount_file_id) REFERENCES mount_files(id)
);

CREATE INDEX IF NOT EXISTS idx_mfv_file ON mount_file_versions(mount_file_id);

CREATE TABLE IF NOT EXISTS shares (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	deleted_at DATETIME
);

CREATE TABLE IF NOT EXISTS share_files (
	id TEXT PRIMARY KEY,
	share_id TEXT NOT NULL,
	path TEXT NOT NULL,
	blob_key TEXT NOT NULL,
	file_size INTEGER NOT NULL,
	file_hash TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	deleted_at DATETIME,
	UNIQUE(share_id, path),
	FOREIGN KEY (share_id) REFERENCES shares(id)
);

PRAGMA user_version = 1;
