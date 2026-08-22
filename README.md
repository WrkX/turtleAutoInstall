# Tortoise-WoW portable (Windows)

MariaDB lives in this folder next to the server binaries. No system install, no
Windows service — `start.bat` brings it up, `stop.bat` kills it.

The game server itself is not in this repo. Build it from tortoise-wow (or grab
the zip the Windows CI workflow uploads) and unpack into `server\`. Client data
goes in `maps\`. You make those yourself; they are not generated here.

## First run

1. Put `mangosd.exe`, `realmd.exe`, the DLLs and the `*.conf.dist` files in
   `server\`.
2. Put `dbc`, `maps`, `vmaps`, `mmaps` in `maps\` (Turtle 1.18.1 / build 7272).
3. Run `setup.bat`. It downloads MariaDB 10.11, pulls SQL from a sibling
   `..\tortoise-wow` checkout if `sql\` is empty, imports everything, and
   writes confs next to the exes.
4. Run `start.bat`.
5. In the mangosd console: `account create <name> <pass>`.

If your source tree is somewhere else, set `TORTOISE_WOW_SRC` in
`portable.local.env` (copy from `portable.env`). Same file for ports, bot
counts, MariaDB URL, etc.

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
sql\                    synced from tortoise-wow
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
binaries follow whatever terms apply to the tortoise-wow build you dropped in.
