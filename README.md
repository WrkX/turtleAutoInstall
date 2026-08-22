# Tortoise-WoW portable (Windows)

MariaDB lives in this folder next to the server binaries. No system install, no
Windows service — `start.bat` brings it up, `stop.bat` kills it.

Client data goes in `maps\` — you provide that yourself; maps come later.
Server binaries and SQL come from the
[tortoise-wow](https://github.com/WrkX/tortoise-wow) GitHub release
(`tortoise-wow-windows-server-*.zip` + `tortoise-wow-sql-*.zip`, same tag).

## First run

1. Put `dbc`, `maps`, `vmaps`, `mmaps` in `maps\` (Turtle 1.18.1 / build 7272).
2. Run `setup.bat`. It downloads the latest server + SQL release and MariaDB
   10.11, imports everything, and writes confs next to the exes.
3. Run `start.bat`.
4. In the mangosd console: `account create <name> <pass>`.

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

Defaults: MySQL on `127.0.0.1:3306` as `mangos`/`mangos`, realm auth 3724,
world 8090, 20 random bots. Raise the bot count once the realm actually works —
a thousand on first boot just makes you wait.

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
