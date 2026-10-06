@echo off
rem Build standalone cfe2cf.exe from Go source (requires Go installed: winget install GoLang.Go)
where go >nul 2>&1
if errorlevel 1 (
    echo [ERROR] Go not found in PATH. Install it: winget install GoLang.Go
    pause
    exit /b 1
)
go build -trimpath -ldflags "-s -w" -o "%~dp0..\bin\cfe2cf.exe" "%~dp0."
if errorlevel 1 (
    echo [ERROR] Build failed.
    pause
    exit /b 1
)
echo Build OK: %~dp0..\bin\cfe2cf.exe
pause
