Needs to match the tortoise-wow commit your binaries came from:

```
create_databases.sql
base\
database_updates\
playerbots\world\          (+ classic\)
playerbots\characters\
```

From a checkout next door:

```
powershell -File tools\sync-sql.ps1 -Source ..\tortoise-wow
```

Or set TORTOISE_WOW_SRC and let setup.bat do it when sql\ is empty.
