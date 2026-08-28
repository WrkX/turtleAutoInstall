@echo off
setlocal
cd /d "%~dp0"

if exist "%~dp0tortoise.exe" (
  "%~dp0tortoise.exe" %*
  exit /b %errorlevel%
)

where go >nul 2>nul
if errorlevel 1 (
  echo tortoise.exe missing - build with: go build -o tortoise.exe .\tui
  echo Or download a release binary next to this script.
  pause
  exit /b 1
)

pushd "%~dp0tui"
go run . %*
set err=%errorlevel%
popd
exit /b %err%
