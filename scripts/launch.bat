@echo off
REM Lorag Launcher for Windows
REM Usage: launch.bat [port]

setlocal enabledelayedexpansion

set "SCRIPT_DIR=%~dp0"
set "PROJECT_ROOT=%SCRIPT_DIR%.."
set "PORT=%~1"
if "!PORT!"=="" set "PORT=8080"
set "HOST=%LORAG_HOST%"
if "!HOST!"=="" set "HOST=localhost"
set "DATA_DIR=%LORAG_DATA_DIR%"
if "!DATA_DIR!"=="" set "DATA_DIR=%PROJECT_ROOT%\data"

echo ==========================================
echo                LORAG
echo      Local RAG + MCP Engine
echo ==========================================
echo.

REM Check if binary exists
if not exist "%PROJECT_ROOT%\bin\lorag.exe" (
    echo Compilation du binaire...
    cd /d "%PROJECT_ROOT%"
    call make build-server
    if errorlevel 1 (
        echo Erreur de compilation
        pause
        exit /b 1
    )
)

REM Create data directory
if not exist "%DATA_DIR%" (
    echo Creation du dossier de donnees: %DATA_DIR%
    mkdir "%DATA_DIR%"
)

REM Create logs directory
if not exist "%DATA_DIR%\logs" (
    mkdir "%DATA_DIR%\logs"
)

echo Demarrage de Lorag...
echo   Host: %HOST%
echo   Port: %PORT%
echo   Data: %DATA_DIR%
echo.

set "LORAG_HOST=%HOST%"
set "LORAG_PORT=%PORT%"
set "LORAG_DATA_DIR=%DATA_DIR%"

echo Serveur demarre !
echo   Web UI:   http://%HOST%:%PORT%
echo   API:      http://%HOST%:%PORT%/api/v1/
echo   MCP:      http://%HOST%:%PORT%/mcp
echo.
echo Appuyez sur Ctrl+C pour arreter
echo.

REM Open browser
start http://%HOST%:%PORT%

REM Start server
"%PROJECT_ROOT%\bin\lorag.exe" server

pause
