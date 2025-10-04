                            @echo off
setlocal EnableDelayedExpansion

echo ========================================
echo Starting Cursor2API Service on Port 5001
echo ========================================
echo.

echo [Step 1/4] Cleaning Go build cache...
go clean -cache -modcache
if errorlevel 1 (
    echo ERROR: Failed to clean cache
    pause
    exit /b 1
)
echo Cache cleaned successfully.
echo.

echo [Step 2/4] Downloading dependencies...
go mod download
if errorlevel 1 (
    echo ERROR: Failed to download dependencies
    pause
    exit /b 1
)
echo Dependencies downloaded successfully.
echo.

echo [Step 3/4] Building cursor2api...
go build -o cursor2api.exe .
if errorlevel 1 (
    echo ERROR: Failed to build cursor2api
    pause
    exit /b 1
)
echo Build completed successfully.
echo.

echo [Step 4/4] Starting cursor2api service...
echo Service will start on http://localhost:5001
echo Press Ctrl+C to stop the service
echo.
cursor2api.exe

pause