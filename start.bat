@echo off
setlocal
cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0tools\start.ps1" %*
if errorlevel 1 (
  echo Start failed.
  pause
  exit /b %errorlevel%
)
