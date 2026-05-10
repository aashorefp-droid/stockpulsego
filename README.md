# StockPulse Go Backend

A Go rewrite of the Python FastAPI backend, prioritizing the scanner endpoint where Go's true concurrency provides the biggest win.

## Status

**Phase 1 — foundation (this milestone, working):**
- HTTP server with chi router, CORS, graceful shutdown
- Alpaca daily/hourly bar client with pagination + retries
- TA math from scratch: SMA, EMA, RSI (Wilder), MACD, ATR, Bollinger, Fib, Support/Resistance
- Macro snapshot endpoint (`GET /api/macro/snapshot`) — uses VIXY as VIX proxy
- Stock analysis endpoint (`GET /api/analysis/{ticker}`)
- Scanner SSE endpoint (`GET /api/scanner/stream`) with goroutine fan-out (12 workers)
- Watchlists endpoint (`GET /api/scanner/watchlists`)
- Dockerfile for Render deployment

**Phase 2 — pending (next session):**
- Options service: bias, strategy, OTM liquid (port from `backend/services/options.py`)
- Earnings service + endpoints + SQLite tracking
- Scheduler with cron jobs + Telegram alerts
- Full analysis pipeline parity (volume profile, FVG patterns, weekly bars, etc.)

## Run locally

```bash
cd go-backend
export ALPACA_API_KEY=...
export ALPACA_API_SECRET=...
export PORT=8000
go run ./cmd/server
```

Test:
```bash
curl http://localhost:8000/health
curl http://localhost:8000/api/macro/snapshot
curl http://localhost:8000/api/analysis/AAPL
curl -N "http://localhost:8000/api/scanner/stream?watchlist=tech"
```

## Deploy to Render

Add a new service in `render.yaml`:

```yaml
- type: web
  name: stockpulse-api-go
  runtime: docker
  rootDir: go-backend
  envVars:
    - key: ALPACA_API_KEY
      sync: false
    - key: ALPACA_API_SECRET
      sync: false
    - key: CORS_ORIGINS
      sync: false
```

## Architecture notes

- **Concurrency**: scanner uses a worker pool (semaphore + WaitGroup) sized to 12 goroutines. Each ticker's analysis is independent — Go's runtime schedules them across cores.
- **TA math**: pure Go, NaN-aware. No external libs. Wilder smoothing for RSI/ATR matches the Python pandas behavior.
- **No yfinance**: Alpaca-only as requested. VIX is approximated by VIXY ETF in the macro endpoint since Alpaca doesn't expose `^VIX`.
- **No SQLite yet**: pending earnings tracker port. Will use `modernc.org/sqlite` (pure Go, no CGO).

## What the Python version still does that this doesn't (yet)

- Options chain analysis (Alpaca options snapshots)
- Earnings calendar discovery (Yahoo screener API)
- Walk-forward backtesting
- APScheduler cron jobs (8:30 AM pre-earnings, 3-6 PM EPS polling)
- Telegram bot integration
- Volume profile / fair value gap detection
- Multi-timeframe analysis using actual hourly bars (currently uses daily as proxy)

These are tractable ports — the foundation here is sized to support them.
