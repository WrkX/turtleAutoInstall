# Portable installer improvements

Actionable follow-ups from the auto installer / updater / server-starter review.
Windows singleplayer Tortoise-WoW portable stack (`tools/*.ps1` + optional TUI).

## P0 — reliability

### 1. Force/update must re-download zips
**Problem:** `fetch-server.ps1`, `fetch-sql.ps1`, and `fetch-maps.ps1` treat `-Force` as “reuse cached zip, re-unpack.” Maps are worst: Drive URLs often normalize to the same filename, so a new share never downloads.

**Do:**
- On `-Force`, delete the cache zip (or skip cache) before download.
- Prefer caching maps by URL hash / ETag / size, not a fixed `tortoise-wow-maps.zip` name.
- Confirm `update.ps1` actually gets new assets after a release or maps URL change.

### 2. Sync realmlist when applying config
**Problem:** Settings / setup write `REALM_ADDRESS` / `WORLD_PORT` into confs, but `tw_logon.realmlist` is only set in `import-databases.ps1`. Address changes leave the client on the old realmlist.

**Do:**
- Upsert realmlist from env whenever confs are applied (setup Apply path, or a small `sync-realmlist.ps1`).
- Point `start.ps1` hint at `REALM_ADDRESS`, not hardcoded `127.0.0.1`.

### 3. Guard against double-start
**Problem:** `start.ps1` always launches realmd/mangosd; TUI disables Start, `start.bat` does not.

**Do:** Exit early if already running, or stop-then-start. Prefer path/PID scoped to this install.

### 4. Fix README MySQL port
**Problem:** README says `127.0.0.1:3306`; `portable.env` defaults to **3307**.

**Do:** Align docs with `MYSQL_PORT` default.

## P1 — first-run UX

### 5. Fail or warn hard if maps are missing
Setup can finish “successfully” when maps URL is empty / fetch skipped. Fail setup (or loud non-zero warning) unless `dbc`/`maps`/`vmaps`/`mmaps` are present under `maps\`.

### 6. Document TUI account as primary
Prefer **Create account** / `tools/create-account.ps1` over mangosd `account create` in README and start hints.

### 7. Download progress
Large MariaDB/maps downloads under the TUI feel stuck. Prefer `curl.exe -L` with progress (already used in parts of maps fetch) for all big fetches.

### 8. Ship `tortoise.exe` in releases
`tui.bat` already expects a binary; without it, users need Go. Add a Windows amd64 build to CI/release.

### 9. Settings form noise
Opening Settings and saving writes placeholders as real values into `portable.local.env`. Prefer empty fields + placeholders so only real overrides are saved.

### 10. Cleaner abort during downloads
TUI `Process.Kill()` on PowerShell can leave orphan curl/mysqld and partial zips. Kill the process tree; remove `.partial` / incomplete cache files.

## P2 — polish / safety

### 11. Scope stop to this install
`stop.ps1` kills any `mangosd`/`realmd` on the machine. Use PID files or executable path under this repo.

### 12. Tighten local credential exposure
MySQL `-p$Password` and account `-ShaPassHash` appear on process argv. Prefer defaults-file / stdin for anything beyond localhost toys. Document: bind localhost only; change defaults before LAN exposure.

### 13. Escape SQL string interpolation
`import-databases.ps1` interpolates realm name / password into SQL. Escape quotes (or otherwise sanitize) so `'` in passwords does not break import.

### 14. Harden `update.ps1` exit checks
`$LASTEXITCODE` after PowerShell scripts can false-fail. Reset before calls or rely on thrown errors / explicit exit codes.

### 15. Minimal CI
- `go test` for `tui/` (hash, env merge).
- Optional Pester smoke for URL/env helpers.
- Build `tortoise.exe` artifact.

### 16. Docs for maps failure modes
Document Drive virus-scan / confirm page, DataNodes API key, and `TORTOISE_WOW_MAPS_ZIP_URL` override in README.

## Out of scope (for now)

- Linux server/MariaDB twins — product is Windows-first; either refuse non-Windows actions early or leave as-is.
- Full installer wrapper — README already notes unpack + `setup.bat` / `-SkipDownload`.

## Suggested order

1. Force/cache re-download (esp. maps)
2. Realmlist sync on Apply + README port
3. Maps-missing / start-already-running guards
4. Account docs + ship `tortoise.exe`
5. Progress, abort cleanup, settings save hygiene
6. Stop scoping, SQL escaping, CI, maps failure docs
