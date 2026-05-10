# install-task.ps1
#
# Registers a Windows Task Scheduler task that runs trigger-snapshot.ps1
# every weekday at 3:00 PM CST (= 4:00 PM ET = 21:00 UTC during standard time).
#
# Run this script ONCE in an elevated PowerShell:
#   Right-click PowerShell → Run as Administrator
#   cd C:\Users\malla\git\streamlit\go-backend\scripts
#   .\install-task.ps1

$TaskName = "StockPulse-Snapshot-3pmCST"
$ScriptPath = Join-Path $PSScriptRoot "trigger-snapshot.ps1"

if (-not (Test-Path $ScriptPath)) {
    Write-Error "Cannot find $ScriptPath"
    exit 1
}

# Remove existing task with the same name (so re-running this updates it)
$existing = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
if ($existing) {
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
    Write-Host "Removed existing task: $TaskName"
}

# Run powershell.exe with the script bypassing the execution policy
$Action = New-ScheduledTaskAction `
    -Execute "powershell.exe" `
    -Argument "-NoProfile -ExecutionPolicy Bypass -File `"$ScriptPath`""

# 3:00 PM local time, Mon-Fri
$Trigger = New-ScheduledTaskTrigger `
    -Weekly -DaysOfWeek Monday,Tuesday,Wednesday,Thursday,Friday `
    -At 3:00PM

# Run whether user is logged in or not, with highest privileges
$Principal = New-ScheduledTaskPrincipal `
    -UserId $env:USERNAME `
    -LogonType S4U `
    -RunLevel Highest

$Settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -StartWhenAvailable `
    -ExecutionTimeLimit (New-TimeSpan -Minutes 5)

Register-ScheduledTask `
    -TaskName $TaskName `
    -Action $Action `
    -Trigger $Trigger `
    -Principal $Principal `
    -Settings $Settings `
    -Description "Triggers daily NYSE/NASDAQ swing snapshot in StockPulse Go backend"

Write-Host ""
Write-Host "Task installed: $TaskName"
Write-Host "  Schedule:   Mon-Fri at 3:00 PM (your local time)"
Write-Host "  Script:     $ScriptPath"
Write-Host "  Log:        $(Join-Path $PSScriptRoot 'trigger-snapshot.log')"
Write-Host ""
Write-Host "To test it manually right now, run:"
Write-Host "  Start-ScheduledTask -TaskName '$TaskName'"
Write-Host ""
Write-Host "To remove later:"
Write-Host "  Unregister-ScheduledTask -TaskName '$TaskName' -Confirm:`$false"
