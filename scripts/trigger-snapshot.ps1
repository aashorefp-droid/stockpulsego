# trigger-snapshot.ps1
#
# Calls the StockPulse Go backend to trigger the NYSE/NASDAQ swing snapshot.
# Designed to be invoked by Windows Task Scheduler at 3:00 PM CST every weekday.
#
# Edit $ApiUrl below to point at your backend:
#   - Localhost:    http://localhost:8000
#   - Render (Go):  https://stockpulse-api-go.onrender.com
#
# Exit codes:
#   0 = trigger accepted (HTTP 202)
#   1 = HTTP error
#   2 = network error

$ApiUrl = "http://localhost:8000"
$Endpoint = "$ApiUrl/api/scanner/snapshot/run"
$LogFile = Join-Path $PSScriptRoot "trigger-snapshot.log"

$timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
"[$timestamp] Triggering snapshot via $Endpoint" | Add-Content $LogFile

try {
    $response = Invoke-WebRequest -Uri $Endpoint -Method Post -TimeoutSec 30 -UseBasicParsing
    "[$timestamp] HTTP $($response.StatusCode): $($response.Content)" | Add-Content $LogFile
    if ($response.StatusCode -eq 202 -or $response.StatusCode -eq 200) {
        exit 0
    }
    exit 1
} catch {
    "[$timestamp] ERROR: $_" | Add-Content $LogFile
    exit 2
}
