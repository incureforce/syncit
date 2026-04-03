# syncit

Small utility - keep files in sync across a few machines driven by **tags**.

## Why this exists

I don't want to setup Terraform, Ansible towers, or other heavy ops stacks in my **homelab**. Also to put my SE-Degree to use outside of my professional Work aka. the hobby projects.

Written in Go 1.26+, Made with Cursor - because it helps me convert my ideas **faster**.

**Work in progress!** 

> APIs and behavior may change. **Use at your own risk** on real data—take backups, try in a throwaway directory first. **Feedback and issues are welcome** (bugs, rough edges, homelab workflows).

**Not** a distributed VCS. **Not** hardened for the public internet—use a trusted LAN or VPN.

**Install Scripts**

Linux: [install.sh](https://raw.githubusercontent.com/incureforce/syncit/refs/heads/main/scripts/install.sh)

```sh
curl -sS https://raw.githubusercontent.com/incureforce/syncit/refs/heads/main/scripts/install.sh | sh
```

---

## Architecture

| Piece | Role |
|--------|------|
| **Server** | SQLite + blob store; catalogs files per mount name + relative path; notifies clients (SSE). |
| **Client** | `~/.syncit/client.db` (override with `SYNCIT_HOME`); local mount roots are **only on the client**—the server sees `(mount name, relative path)`. |

Paths you track are always **relative to a named mount**. Nearest mount wins when paths overlap (longest root prefix).

---

## Build

```bash
go build -o syncit ./cmd/syncit
```

---

## Server

```bash
syncit run --addr :8080 --data-dir ./syncit-data
```

Listens for HTTP (client registration, blobs, sync APIs). Point clients at `http://host:port` (no trailing slash required; client normalizes).

---

## Client: first run

```bash
export SYNCIT_HOME=~/.syncit   # optional; default is ~/.syncit

syncit init http://127.0.0.1:8080
syncit mount add work ~/projects/myrepo
syncit file add ~/projects/myrepo/README.md
syncit push
```

`init` creates the client DB and registers the client UUID with the server.

---

## Tracking & sync

| Command | Purpose |
|---------|---------|
| `syncit init <server-url>` | Initialize local client DB and register this client with the server. |
| `syncit mount add <name> <path>` / `syncit mount del <name>` / `syncit mount ls [-a]` | Manage named mount roots and sync mount names to server. |
| `syncit file add <path> [tags...]` | Track a file or recurse a directory; optional **file tags** filter which clients receive the file (see below). |
| `syncit file del [-f] <path> [path...]` | Untrack local file(s) and propagate delete/untrack state to server (`-f` also removes local files). |
| `syncit file ls` | Tracked files and status. |
| `syncit file inspect <path>` | Show detailed tracked-file state (tags, hashes, local/remote versions, conflict flag). |
| `syncit file get <path>` | Pull server version over local (conflict resolution). |
| `syncit info` | Mounts and client tags. |
| `syncit tag ls` / `syncit tag add ...` / `syncit tag del ...` | List and manage this client's tag set (synced to server). |
| `syncit version` | Print embedded build version tag (or `development`). |
| `syncit upgrade` | Download and install latest release for current platform. Stops if daemon is running or already on latest tag. |

Upload local changes:

```bash
syncit push                    # shorthand for syncit sync push
syncit sync push
```

Download everything the server knows for your mounts (repair / full catalog):

```bash
syncit sync pull
```

Stay up to date via server events:

```bash
syncit sync daemon
```

`sync pull` takes the data-directory lock; **`push` does not**, so you can run **`syncit push` while `sync daemon` is running** (SQLite `busy_timeout` coordinates writes).

### Push options

```bash
# Re-hash **already tracked** files under optional paths, then push if needed (-a ignores untracked paths)
syncit push -a
syncit push -a ./docs ./src

# Limit to paths or globs (paths must fall under a mount; ** supported via doublestar)
syncit push '**/README.md'
```

Conflicts: paths marked conflict are skipped by push/pull until you resolve (e.g. `syncit file get -y <path>` for remote wins).

### Utility commands

```bash
syncit version
syncit upgrade
syncit wipe
```

- `version`: prints embedded release tag from `internal/cli/version.tag` (or `development` when no release tag is embedded).
- `upgrade`: updates to latest GitHub release and installs into `~/.syncit/<tag>/...`, with launcher in `~/.syncit/bin`.
- `wipe`: removes local syncit state directory after confirmation.

---

## Tags (conditional sync)

This is the homelab “only these servers” knob:

- **Mount** = allowed on-disk roots per machine.
- **File tags** (`syncit file add path tag1 tag2 …`) = optional filter: a client receives that file only if **every** file tag is in that client’s tag set. Omit file tags → everyone subscribed to that mount path can sync it.
- **Client tags** (`syncit tag add …`) = what each machine is subscribed to, on top of mounts.

Full rules and edge cases: [`docs/design_v1.md`](docs/design_v1.md).

---

## Not implemented yet

- `syncit share` (stubs / phase 2)
