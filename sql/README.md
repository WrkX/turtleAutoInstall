Filled by `setup.bat` from the matching
[tortoise-wow](https://github.com/WrkX/tortoise-wow) release
(`tortoise-wow-sql-*.zip` — base, migrations, playerbots).

Fallback from a checkout next door:

```
powershell -File tools\sync-sql.ps1 -Source ..\tortoise-wow
```

Or set `TORTOISE_WOW_SRC` / `TORTOISE_WOW_SQL_ZIP_URL` in `portable.local.env`.
