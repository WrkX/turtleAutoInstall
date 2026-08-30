Turtle WoW 1.18.1 (build 7272) client data:

```
dbc\
maps\
vmaps\
mmaps\
```

Filled by `tools\fetch-maps.ps1` from `conf/maps-url.txt` (Google Drive share
link or direct zip; a cached URL copy is refreshed from GitHub on each fetch).
Override with `TORTOISE_WOW_MAPS_ZIP_URL` in `portable.local.env`. Not checked
in.

The zip must contain all four directories, either at its top level or below one
containing folder. Do not point the setting at an HTML download/preview page.
If a previous download was interrupted, run **Fetch maps** again (or delete
`tools\.cache\maps-*.zip`); setup verifies that every directory is non-empty
before it marks the install complete.

If Google Drive serves a virus-scan/confirm page, make sure the share is
“Anyone with the link can view”; large files may still require a direct/raw zip
host. DataNodes page links require `DATANODES_API_KEY` in
`portable.local.env`. A direct zip URL can always be supplied through
`TORTOISE_WOW_MAPS_ZIP_URL`. Setup is incomplete until all four directories are
present.
