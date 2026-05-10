@echo off
:: Launcher for the Go backend used by bounce.bat.
:: Sets env vars cleanly (no trailing whitespace) and starts the server.

set "PATH=C:\Program Files\Go\bin;%PATH%"
set "ALPACA_API_KEY=AKBTFWNEQOAQ6YTHVSEBZHSERS"
set "ALPACA_API_SECRET=9r3XFuhPUcaouy4Vin38D3zeT8ksYxM8tmvj6PwhQzDF"
set "POLYGON_API_KEY=q4Sx_c3RB9LeUkX4_efYSjFqi4dWtBHz"
set "PORT=8000"

cd /d "%~dp0"

echo.
echo Starting StockPulse Go backend on port %PORT%...
echo.

go run ./cmd/server
