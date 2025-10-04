@echo off
REM ============================================================================
REM One-Click Startup Script for Windows
REM ============================================================================
REM Description: Production-ready startup script with comprehensive checks
REM Platform: Windows 10/11 (cmd.exe)
REM Usage: run.bat [dev|prod|stop|restart|logs|status]
REM ============================================================================

setlocal EnableDelayedExpansion

REM Project configuration
set PROJECT_NAME=cursor2api

REM Parse command
set COMMAND=%1
if "%COMMAND%"=="" set COMMAND=prod

REM Color codes (Windows 10+)
set "RED=[91m"
set "GREEN=[92m"
set "YELLOW=[93m"
set "BLUE=[94m"
set "NC=[0m"

REM Main dispatcher
if /I "%COMMAND%"=="dev" goto START_DEV
if /I "%COMMAND%"=="prod" goto START_PROD
if /I "%COMMAND%"=="stop" goto STOP_SERVICES
if /I "%COMMAND%"=="restart" goto RESTART_SERVICES
if /I "%COMMAND%"=="logs" goto SHOW_LOGS
if /I "%COMMAND%"=="status" goto SHOW_STATUS
if /I "%COMMAND%"=="help" goto SHOW_USAGE
if /I "%COMMAND%"=="-h" goto SHOW_USAGE
if /I "%COMMAND%"=="--help" goto SHOW_USAGE

echo %RED%[ERROR]%NC% Unknown command: %COMMAND%
goto SHOW_USAGE

:START_DEV
call :LOG_INFO "Starting in development mode..."
call :CHECK_DOCKER || exit /b 1
call :CHECK_COMPOSE || exit /b 1
call :CHECK_ENV || exit /b 1
call :GET_COMPOSE_CMD
%COMPOSE_CMD% build --no-cache
%COMPOSE_CMD% up -d
timeout /t 3 >nul
call :SHOW_STATUS
exit /b 0

:START_PROD
call :LOG_INFO "Starting in production mode..."
call :CHECK_DOCKER || exit /b 1
call :CHECK_COMPOSE || exit /b 1
call :CHECK_ENV || exit /b 1
call :GET_COMPOSE_CMD
%COMPOSE_CMD% up -d
timeout /t 3 >nul
call :SHOW_STATUS
exit /b 0

:STOP_SERVICES
call :LOG_INFO "Stopping services..."
call :GET_COMPOSE_CMD
%COMPOSE_CMD% down
call :LOG_SUCCESS "Services stopped"
exit /b 0

:RESTART_SERVICES
call :STOP_SERVICES
timeout /t 2 >nul
call :START_PROD
exit /b 0

:SHOW_LOGS
call :LOG_INFO "Showing logs (Ctrl+C to exit)..."
call :GET_COMPOSE_CMD
%COMPOSE_CMD% logs -f --tail=100
exit /b 0

:SHOW_STATUS
call :GET_COMPOSE_CMD
echo.
%COMPOSE_CMD% ps
echo.
for /f "tokens=2 delims==" %%i in ('findstr /B "PORT=" .env 2^>nul') do set PORT=%%i
if "!PORT!"=="" set PORT=8000
call :LOG_SUCCESS "Service running at:"
echo   %GREEN%??%NC% http://localhost:!PORT!
echo.
exit /b 0

:SHOW_USAGE
echo %GREEN%%PROJECT_NAME% - One-Click Startup Script%NC%
echo.
echo %BLUE%Usage:%NC% run.bat [COMMAND]
echo.
echo %BLUE%Commands:%NC%
echo     dev        Start in development mode (rebuild)
echo     prod       Start in production mode (default)
echo     stop       Stop all services
echo     restart    Restart services
echo     logs       Show and follow logs
echo     status     Show service status
echo     help       Show this help
echo.
echo %BLUE%Examples:%NC%
echo     run.bat              # Start production mode
echo     run.bat dev          # Start development mode
echo     run.bat logs         # View logs
echo.
echo %BLUE%Configuration:%NC%
echo     Edit .env and set API_KEY
echo     Generate: powershell -Command "$key = -join ((48..57 + 65..90 + 97..122) | Get-Random -Count 40 | ForEach-Object {[char]$_}); Write-Host 'sk-'$key"
echo.
exit /b 0

REM ============================================================================
REM Helper Functions
REM ============================================================================

:LOG_INFO
echo %BLUE%[INFO]%NC% %~1
exit /b 0

:LOG_SUCCESS
echo %GREEN%[SUCCESS]%NC% %~1
exit /b 0

:LOG_WARNING
echo %YELLOW%[WARNING]%NC% %~1
exit /b 0

:LOG_ERROR
echo %RED%[ERROR]%NC% %~1
exit /b 0

:CHECK_DOCKER
call :LOG_INFO "Checking Docker..."
docker --version >nul 2>&1
if errorlevel 1 (
    call :LOG_ERROR "Docker not installed. Visit: https://docs.docker.com/desktop/install/windows-install/"
    exit /b 1
)
docker info >nul 2>&1
if errorlevel 1 (
    call :LOG_ERROR "Docker Desktop not running"
    exit /b 1
)
call :LOG_SUCCESS "Docker is ready"
exit /b 0

:CHECK_COMPOSE
call :LOG_INFO "Checking Docker Compose..."
docker compose version >nul 2>&1
if not errorlevel 1 (
    call :LOG_SUCCESS "Docker Compose is ready"
    exit /b 0
)
docker-compose --version >nul 2>&1
if not errorlevel 1 (
    call :LOG_SUCCESS "Docker Compose is ready"
    exit /b 0
)
call :LOG_ERROR "Docker Compose not installed"
exit /b 1

:CHECK_ENV
call :LOG_INFO "Checking environment configuration..."
if not exist .env (
    if exist .env.example (
        copy .env.example .env >nul
        call :LOG_WARNING "Created .env from template. Please configure API_KEY!"
        call :LOG_INFO "Generate: powershell -Command \"$key = -join ((48..57 + 65..90 + 97..122) | Get-Random -Count 40 | ForEach-Object {[char]$_}); Write-Host 'sk-'$key\""
        exit /b 1
    )
    call :LOG_ERROR ".env file missing"
    exit /b 1
)
findstr /B "API_KEY=CHANGE_ME" .env >nul 2>&1
if not errorlevel 1 (
    call :LOG_ERROR "API_KEY invalid or not configured"
    call :LOG_INFO "Generate: powershell -Command \"$key = -join ((48..57 + 65..90 + 97..122) | Get-Random -Count 40 | ForEach-Object {[char]$_}); Write-Host 'sk-'$key\""
    exit /b 1
)
findstr /B "API_KEY=sk-" .env >nul 2>&1
if errorlevel 1 (
    call :LOG_ERROR "API_KEY must start with 'sk-'"
    exit /b 1
)
call :LOG_SUCCESS "Environment configuration valid"
exit /b 0

:GET_COMPOSE_CMD
docker compose version >nul 2>&1
if not errorlevel 1 (
    set COMPOSE_CMD=docker compose
) else (
    set COMPOSE_CMD=docker-compose
)
exit /b 0
