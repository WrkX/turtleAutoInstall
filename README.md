# Tortoise-WoW portable (Windows)

The usual install is a single `tortoise.exe` from GitHub Releases. Put it in an
empty folder and run it. On first launch it unpacks the helper scripts and
default config next to the exe; **Full setup** then downloads MariaDB, the
[tortoise-wow](https://github.com/WrkX/tortoise-wow) server + SQL zips, and maps
when `conf/maps-url.txt` has a zip URL.

MariaDB lives in that folder next to the server binaries. No system install, no
Windows service — **Start realm** brings it up, **Stop realm** kills it.

Client data (`dbc`, `maps`, `vmaps`, `mmaps`) is fetched from the URL in
`conf/maps-url.txt` (public Google Drive share link or a raw https zip).
Override with `TORTOISE_WOW_MAPS_ZIP_URL` in `portable.local.env`.

## First run

1. Download `tortoise.exe` from the latest GitHub release into a new folder.
2. Run it and choose **Full setup**. It downloads the latest server + SQL
   release, MariaDB 10.11, and maps when `conf/maps-url.txt` has a zip URL.
3. Choose **Start realm**.
4. Use **Create account** (`a`). The helper updates an existing account safely
   and prints the realmlist. The mangosd `account create` console command
   remains a fallback.

Replacing `tortoise.exe` with a newer release refreshes the bundled scripts.
Your `portable.local.env`, databases, and maps stay put.

From this git checkout you can still run `tui.bat`, or build with
`go build -o tortoise.exe ./tui` (set `GOOS=windows` `GOARCH=amd64` from Linux).
`tui.bat` prefers `tortoise.exe` and falls back to `go run` when it is absent.

Override downloads in `portable.local.env` (copy from `portable.env`):
`TORTOISE_WOW_RELEASE` to pin a tag like `0.1.1`, direct zip URLs, or
`TORTOISE_WOW_SRC` if you prefer copying SQL from a local checkout instead.
Same file for ports, bot counts, MariaDB URL, etc.

## What sits where

After the first launch (the exe writes the files it needs):

```
tortoise.exe            launcher (the only file you download)
portable.env            defaults (created if missing)
portable.local.env      your overrides (gitignored)
conf\my.ini.template
tools\                  scripts unpacked from the exe
mariadb\                filled by setup
data\mysql\             datadir
server\                 binaries
sql\                    from release zip (or local checkout)
maps\                   client data
logs\
```

Defaults: MySQL on `127.0.0.1:3307` as `mangos`/`mangos`, realm auth 3724,
world 8090, 20 random bots. Raise the bot count once the realm actually works —
a thousand on first boot just makes you wait.

## Maps troubleshooting

`Full setup` requires all four client-data directories (`dbc`, `maps`, `vmaps`,
and `mmaps`) under `maps\`. Set `TORTOISE_WOW_MAPS_ZIP_URL` in
`portable.local.env` to override `conf/maps-url.txt`, then run **Fetch maps**.
If setup stops with a missing-maps error, fix the URL or place the four folders
under `maps\` and rerun setup; it is safe to rerun because completed downloads
and database imports are skipped.

Google Drive may return a virus-scan/confirm page for a large file. The share
must be public (“Anyone with the link can view”); if the confirm page still
blocks the download, use a direct/raw zip URL (or host the archive elsewhere).
DataNodes free-page links also need a `DATANODES_API_KEY` in
`portable.local.env`; alternatively set the maps override to a direct zip URL.
An empty or invalid URL leaves maps incomplete and setup will report the missing
directories.

The TUI shortcuts are available from the home screen: `a` creates an account,
`c` copies `set realmlist ...`, `s` opens settings, `u` opens the update action,
and `l` opens the logs folder. Use **Fetch maps** after changing a maps URL; use
**Apply config** after changing ports or the realm address.

Keep the portable database bound to localhost unless you have deliberately
changed the defaults and secured the credentials; the setup scripts are aimed
at a local single-player realm.

Import follows the same recipe as INSTALL-WINDOWS.md in the source tree
(`create_databases`, `sql/base`, migrations with `--force` + mark applied,
playerbot SQL). To wipe and reload, use **Reimport databases** in the TUI
(or `powershell -File tools\setup.ps1 -ForceReimport`).

MariaDB is GPL — keep its license files if you redistribute MariaDB. Server
binaries come from the tortoise-wow CI release; follow whatever terms apply.
