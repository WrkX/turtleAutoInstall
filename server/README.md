Put the Windows server build here. Either the flat folder from
`cmake --install`, or the `tortoise-wow-windows-server-*.zip` artifact from
the source repo's Windows Build workflow.

Needs at least mangosd.exe, realmd.exe, the usual DLLs (libmySQL, ACE, Boost,
OpenSSL), and the `*.conf.dist` templates. setup.bat turns the templates into
real confs; leave live `*.conf` out of git.

dbc / maps / vmaps / mmaps go in `..\maps\`, not in this folder.
