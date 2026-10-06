@echo off
setlocal

rem ============================================================
rem Drop a .cfe file onto this bat file.
rem The exe finds the 1C:Enterprise platform on its own and
rem creates the .cf file next to the source .cfe file.
rem ============================================================

if "%~1" == "" (
    echo [ERROR] No file passed.
    echo Drag and drop a .cfe file onto this bat file.
    echo.
    pause
    exit /b
)

if not exist "%~1" (
    echo [ERROR] File not found: "%~1"
    pause
    exit /b
)

"%~dp0cfe2cf.exe" "%~f1"
set "RC=%errorlevel%"

if not "%RC%" == "0" (
    echo.
    echo [ERROR] Conversion failed, exit code %RC%.
)
pause
exit /b %RC%
