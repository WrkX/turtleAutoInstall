Filled automatically by `setup.bat` from the latest
[tortoise-wow](https://github.com/WrkX/tortoise-wow) GitHub release
(`tortoise-wow-windows-server-*.zip`). You can also drop a flat `cmake --install`
folder here by hand, or pin a release in `portable.local.env`.

Needs at least mangosd.exe, realmd.exe, the usual DLLs (libmySQL, ACE, Boost,
OpenSSL), and the `*.conf.dist` templates. setup.bat turns the templates into
real confs; leave live `*.conf` out of git.

dbc / maps / vmaps / mmaps go in `..\maps\`, not in this folder.
