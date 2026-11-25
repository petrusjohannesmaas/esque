# Esque

Esque backs up SQLite databases to S3, encrypted with AES-256. It is one Go binary with a terminal interface. A project holds one database, a passphrase and an optional S3 setup, so one install can back up several apps without a script per app.

Esque needs neither `sqlite3` nor `openssl`. It copies a live database with SQLite's backup call, so data still in the `-wal` file is included, and it encrypts the copy in the same format as `openssl enc -aes-256-cbc -salt -pbkdf2 -iter 100000`.

## Install

With Go 1.27.4 or later:

```bash
go install github.com/petrusjohannesmaas/esque@latest
```

The binary goes to `$(go env GOPATH)/bin`, which is usually `~/go/bin`. From a checkout you can also build it:

```bash
git clone https://github.com/petrusjohannesmaas/esque.git
cd esque
go build -o esque .
```

Then start it with `esque`.

## Config file

Esque keeps its projects in `~/.config/esque/config.json`, or under `$XDG_CONFIG_HOME` when that is set. The folder is mode 0700 and the file mode 0600, so only your user can read them.

The file holds each project's passphrase and S3 secret key in plain text. Headless servers have no keyring, so the file permissions are what protect them. Keep the file out of backups and version control.

The passphrase is set when you add a project and cannot be changed later. Older backups are encrypted with it, so changing it would leave them unreadable. Open the project and choose Reveal passphrase to copy it into a password manager; it hides again on the next key press.

## Add a project

Start Esque and choose Add project, either with `a` or with enter on the last row of the menu. Type a name, the directory that holds the database, and a passphrase. Press tab to move between fields and enter to save. A name may use letters, digits, `-`, `_` and `.`.

Esque finds SQLite files by their file header. If the directory holds one, it is chosen for you. If it holds several, you pick one from a list. If it holds none, Esque says so and adds nothing.

A project backs up one database file. To change the passphrase or the database file, remove the project and add it again.

## Set up S3

Open the project and choose S3 setup. Fill in the endpoint, region, bucket, an optional prefix, the access key ID and the secret key. Any S3-compatible service works, and an endpoint without a scheme uses https.

The secret key is hidden while you type and after you save. Leave the field empty when you edit the setup to keep the current key.

S3 is optional when you add a project. Until it is set up, Back up, List backups and Restore say to set up S3 first.

## Back up

Open the project and choose Back up. The name field holds the project name and the current time, such as `blog-2026-11-25_14-30-00`. Accept it or edit it, then press enter.

Esque snapshots the database, encrypts the snapshot with the project's passphrase and uploads it as `<name>.sqlite.enc` under the prefix. When it finishes, the screen shows the backup's name and size. A name that already exists under the prefix is refused, so a backup never overwrites an older one.

## List and restore

Choose List backups to see each backup's name, size and date, newest first. Select a backup and press enter, then choose a folder. The default is `<project directory>/restored/`.

Esque downloads the backup, decrypts it, runs SQLite's integrity check and writes `<name>.sqlite` into the folder. The live database is never touched, and an existing file in the target folder is not overwritten. If the backup cannot be decrypted with the project's passphrase or fails the integrity check, Esque shows one message and writes nothing.

## Restore with plain openssl

A backup is in the format that `openssl enc -aes-256-cbc -salt -pbkdf2 -iter 100000` writes, so a machine with openssl can restore one without Esque. Download `<name>.sqlite.enc` and run:

```bash
openssl enc -d -aes-256-cbc -salt -pbkdf2 -iter 100000 \
  -in blog-2026-11-25_14-30-00.sqlite.enc \
  -out blog-2026-11-25_14-30-00.sqlite
```

openssl asks for the project's passphrase and writes the database. This cipher mode has no tamper check, so run SQLite's `PRAGMA integrity_check` on the result if you have `sqlite3` installed.

## Remove a project

Open the project, choose Remove and confirm with `y`. The project leaves the list along with its passphrase and S3 setup. The database file and the backups in S3 are left alone.

## Keys

In the main menu, use the arrow keys or `j` and `k` to move, enter to open, `a` to add a project and `q` to quit. In a project or form, enter confirms and esc goes back. Ctrl+C quits from anywhere.

## Sample database

`test_database.sqlite` in the repository is a small database with a `users` table. Copy it into a directory of its own and add that directory as a project to try a backup and restore.
