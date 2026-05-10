# Local Snapshot Scheduling

Run the StockPulse swing-universe snapshot daily from your Windows PC instead of relying on Render's cron.

## How it works

1. **`trigger-snapshot.ps1`** — calls `POST /api/scanner/snapshot/run` on your Go backend. The backend kicks off the scan in a background goroutine and returns immediately. Logs to `trigger-snapshot.log` next to the script.
2. **`install-task.ps1`** — registers a Windows Task Scheduler task that runs the trigger script every weekday at **3:00 PM local time**.

## Setup (one-time)

1. **Edit `trigger-snapshot.ps1`** — change `$ApiUrl` to your backend:
   - Local Go server: `http://localhost:8000`
   - Render Go service: `https://stockpulse-api-go.onrender.com`

2. **Open PowerShell as Administrator** (Start → search PowerShell → right-click → Run as Administrator)

3. **Install the task:**
   ```powershell
   cd C:\Users\malla\git\streamlit\go-backend\scripts
   .\install-task.ps1
   ```

4. **Test it right now:**
   ```powershell
   Start-ScheduledTask -TaskName 'StockPulse-Snapshot-3pmCST'
   ```
   Then check `trigger-snapshot.log` for the result.

## Important

- Your backend (local or Render) **must be reachable** at the configured URL when the task fires
- The Go server's internal cron also fires at 3:00 PM CST when running, so this is mostly a backup if the server is asleep on Render's free tier
- The task is configured to run **whether you're logged in or not** (it stores your credentials securely)
- It will run on battery and **wake the PC** if needed (subject to your power settings — adjust if you don't want this)

## To remove later

```powershell
Unregister-ScheduledTask -TaskName 'StockPulse-Snapshot-3pmCST' -Confirm:$false
```

## Time zone notes

The task triggers at **3:00 PM local time**. If your machine is set to a non-CST timezone, the snapshot will fire at the wrong moment. Two ways to handle:

- **Easy:** rely on the Go backend's internal cron (always uses CST regardless of host TZ) — keep this task as a keepalive only
- **Precise:** edit `install-task.ps1` to compute a UTC time. For 3 PM CST = 21:00 UTC (CDT) / 21:00 UTC (CST is UTC-6 — actually 21:00 UTC during CST, 20:00 UTC during CDT). If your PC is in CST/CDT this just works.
