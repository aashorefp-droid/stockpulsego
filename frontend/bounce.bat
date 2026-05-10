@echo off
setlocal

rem Keep one canonical launcher at the chatgpt project root.
set "ROOT=%~dp0.."
for %%I in ("%ROOT%") do set "ROOT=%%~fI"

call "%ROOT%\bounce.bat"
