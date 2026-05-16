@echo off
title ScoreBot
echo ========================================
echo   ScoreBot - Score Query Service
echo   PC:    http://localhost:8080
echo   Phone: http://192.168.1.8:8080
echo   Press Ctrl+C twice to stop
echo ========================================
echo.

set DATA_STORE=sqlite
set CHAT_ADAPTER=http
set API_LISTEN=0.0.0.0:8080
set SQLITE_STORE_PATH=data.sqlite

:loop
echo [%date% %time%] Starting service...
scorebot.exe
echo [%date% %time%] Service stopped. Restarting in 5s...
timeout /t 5 /nobreak >nul
goto loop
