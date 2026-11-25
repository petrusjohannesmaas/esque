# Esque design

Esque is one Go binary with a Bubble Tea terminal interface. It backs up the SQLite database of each configured project, encrypted, to that project's S3 location, and lists and restores those backups. It needs neither `sqlite3` nor `openssl` installed.

## Modules

All packages live under `internal/`, and `main.go` at the root wires them together.

- `config` loads and saves the project list. A project has a name, a directory, one database file name in that directory, a passphrase and an optional S3 setup. Names are unique.
- `sqlitefile` finds SQLite files in a directory by their 16-byte file header, takes a snapshot of a live database with SQLite's online backup call (`modernc.org/sqlite`, `NewBackup` through `sql.Conn.Raw`), and runs `PRAGMA integrity_check` on a file.
- `crypt` encrypts and decrypts in the `openssl enc -aes-256-cbc -salt -pbkdf2 -iter 100000` format: the `Salted__` header, an 8-byte salt, PBKDF2-HMAC-SHA256 with 100000 iterations for the key and IV, and PKCS#7 padding.
- `remote` is the S3 port. `remote.Store` has Put, Exists, List and Get, with keys relative to the project's prefix. The production adapter uses `minio-go`. It turns S3 errors into `remote.ErrAccessDenied`, `remote.ErrUnreachable` and `remote.ErrNotFound`. `remote/remotetest` holds the in-memory adapter and the contract suite that both adapters run.
- `backup` runs the pipelines. Backup goes from snapshot to encryption to upload as `<name>.sqlite.enc`. List returns backups newest first. Restore goes from download to decryption to the integrity check, then writes `<name>.sqlite` into a chosen folder. It works on temporary files and removes them on any failure, so a failed backup uploads nothing and a failed restore writes nothing.
- `tui` holds the Bubble Tea models: main menu, add project, project view, S3 form, backup, list and restore. It takes the config path, a function that opens a `remote.Store` from an S3 setup, and a clock.

## Data

The config is JSON in `os.UserConfigDir()/esque/config.json`, usually `~/.config/esque/config.json`. The file is mode 0600 and the folder 0700. It holds passphrases and S3 secret keys in plain text, because headless servers have no keyring.

```json
{"projects": [{"name": "blog", "dir": "/srv/blog", "db": "blog.sqlite", "passphrase": "...",
  "s3": {"endpoint": "https://s3.example.com", "region": "af-south-1", "bucket": "backups",
         "prefix": "blog", "access_key_id": "...", "secret_key": "..."}}]}
```

An endpoint without a scheme means https.

## Rules

- Esque never opens the live database for writing. Snapshots read it through the backup call, and restores write a new file only.
- A project name and a backup name use only letters, digits, `-`, `_` and `.`, and are not empty. The default is `<project>-YYYY-MM-DD_HH-MM-SS` in local time. Esque refuses a name that already exists under the prefix.
- A restore refuses to overwrite an existing file in the target folder.
- Only `.sqlite.enc` objects directly under the prefix count as backups.
- The `tui` screens change the config only through `Model.change`, which saves a copy and keeps it only when the save succeeds, so memory never gets ahead of the file. (25 November 2026, #1)

## Tests

- `go test` for everything. The `tui` tests drive the models with `teatest` (`github.com/charmbracelet/x/exp/teatest`) at a fixed terminal size with the ASCII colour profile, a fixed clock and the in-memory store.
- The minio adapter runs the `remotetest` contract suite against `gofakes3` in an `httptest` server.
- One test: `go test ./internal/<pkg> -run TestName`
- All suites, through the queue: `test-queue submit light 'go mod download && go vet ./... && go test ./...'`

### Testing against Garage

Garage v2.6.0 is the real S3 service Esque has been tested with (25 November 2026, #15). `deploy/garage.yaml` runs it in the `esque` namespace in k3s: one node, replication factor 1, a 1 GiB volume and region `garage`. The RPC secret comes from the `garage-rpc` Secret, so the manifest holds no secrets. Set it up once:

```bash
kubectl create namespace esque
kubectl -n esque create secret generic garage-rpc --from-literal=secret="$(openssl rand -hex 32)"
kubectl apply -f deploy/garage.yaml
kubectl -n esque rollout status deploy/garage
g="kubectl -n esque exec deploy/garage -- /garage"
$g layout assign -z dc1 -c 1G "$($g node id -q | cut -d@ -f1)"
$g layout apply --version 1
$g bucket create esque-backups
$g key create esque-key          # prints the key ID and secret key
$g bucket allow --read --write --owner esque-backups --key esque-key
```

Reach it with `kubectl -n esque port-forward svc/garage 8117:3900 --address 127.0.0.1` (port 8117 is esque's in `dev-port`). A port-forward ends with its pod, so start it again after the pod is replaced. Run Esque with `XDG_CONFIG_HOME` set to a spare folder so the test project stays out of the real config, and give the project endpoint `http://127.0.0.1:8117`, region `garage`, bucket `esque-backups` and the key from `garage key create`. `kubectl delete namespace esque` removes Garage and its data.
