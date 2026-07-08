$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$backend = Join-Path $root "backend"
$frontend = Join-Path $root "frontend"

Start-Process powershell -WindowStyle Hidden -WorkingDirectory $backend -ArgumentList @(
  "-NoExit",
  "-Command",
  '$env:GOPROXY="https://goproxy.cn,direct"; go run ./cmd/server'
)

Start-Process powershell -WindowStyle Hidden -WorkingDirectory $frontend -ArgumentList @(
  "-NoExit",
  "-Command",
  "pnpm dev"
)

Write-Host "Backend:  http://127.0.0.1:8080"
Write-Host "Frontend: http://127.0.0.1:5173"
