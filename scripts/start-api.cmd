@echo off
setlocal
set "ROOT=%~dp0.."
set "CONFIG=%ROOT%\config\api.local.json"
if not exist "%CONFIG%" set "CONFIG=%ROOT%\config\api.example.json"
if exist "%ROOT%\uwp-tcp-con.exe" (
  "%ROOT%\uwp-tcp-con.exe" --api --api-config "%CONFIG%"
) else if exist "%ROOT%\uwp-tcp-con" (
  "%ROOT%\uwp-tcp-con" --api --api-config "%CONFIG%"
) else (
  cd /d "%ROOT%" && go run ./cmd/uwp-tcp-con --api --api-config "%CONFIG%"
)
