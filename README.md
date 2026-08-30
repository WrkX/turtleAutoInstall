# Tortoise-WoW portable (Windows)

MariaDB lives in this folder next to the server binaries. No system install, no
Windows service — `start.bat` brings it up, `stop.bat` kills it.

Client data (`dbc`, `maps`, `vmaps`, `mmaps`) is fetched from the URL in
`conf/maps-url.txt` (public Google Drive share link or a raw https zip). That
file is git-tracked; setup/fetch-maps also checks a cached copy refreshed from
GitHub so a new URL in the repo is used without a git pull. Override with
`TORTOISE_WOW_MAPS_ZIP_URL` in `portable.local.env`.
Server binaries and SQL come from the
[tortoise-wow](https://github.com/WrkX/tortoise-wow) GitHub release
(`tortoise-wow-windows-server-*.zip` + `tortoise-wow-sql-*.zip`, same tag).

## First run

1. Run `setup.bat` (or `tui.bat` → Full setup). It downloads the latest server +
   SQL release, MariaDB 10.11, and maps when `conf/maps-url.txt` has a zip URL.
2. Run `start.bat` (or **Start realm** in the TUI).
3. Use **Create account** in the TUI, or run the primary account helper and
   enter the password at its private prompt:
   `powershell -File tools\create-account.ps1 -Username <name>`.
   The helper updates an existing account safely and prints the realmlist. The
   mangosd `account create` console command remains a fallback.

Optional: `tui.bat` opens a Charm/Bubble Tea dashboard (install, update, start/stop,
database reimport, settings) that shells out to the same `tools\*.ps1` scripts. Build with
`go build -o tortoise.exe .\tui` (or `GOOS=windows GOARCH=amd64` from Linux).
Tagged GitHub releases also include a prebuilt `tortoise-wow-tui-windows-amd64.zip`.
Extract `tortoise.exe` and `tui.bat` into this portable root; `tui.bat` will prefer
the binary and falls back to `go run` only when it is absent.

Override downloads in `portable.local.env` (copy from `portable.env`):
`TORTOISE_WOW_RELEASE` to pin a tag like `0.1.1`, direct zip URLs, or
`TORTOISE_WOW_SRC` if you prefer copying SQL from a local checkout instead.
Same file for ports, bot counts, MariaDB URL, etc.

## What sits where

```
setup.bat  start.bat  stop.bat
portable.env            defaults
portable.local.env      your overrides (gitignored)
conf\my.ini.template
tools\                  the actual scripts
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

`setup.bat` requires all four client-data directories (`dbc`, `maps`, `vmaps`,
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
playerbot SQL). To wipe and reload:

```
powershell -File tools\setup.ps1 -ForceReimport
```

When you wrap this in an installer later, have it unpack the tree and run
`setup.bat`. If you ship MariaDB inside the installer, call
`tools\setup.ps1 -SkipDownload` instead.

MariaDB is GPL — keep its license files if you redistribute the zip. Server
binaries come from the tortoise-wow CI release; follow whatever terms apply.
