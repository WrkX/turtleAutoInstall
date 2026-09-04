@echo off
setlocal
cd /d "%~dp0"

where go >nul 2>nul
if not errorlevel 1 (
  go build -o tortoise.exe ./tui
  if errorlevel 1 (
    echo build failed
    pause
    exit /b 1
  )
)

if exist "%~dp0tortoise.exe" (
  "%~dp0tortoise.exe" %*
  exit /b %errorlevel%
)

echo tortoise.exe missing - install Go, or download a release binary next to this script.
pause
exit /b 1
