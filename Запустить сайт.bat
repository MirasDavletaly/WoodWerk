@echo off
chcp 65001 >nul
title WOODWERK - сервер сайта
cd /d "%~dp0"

set EXE=server\build\woodwerk-windows-amd64.exe

if not exist "%EXE%" (
    echo.
    echo Не найден файл %EXE%
    echo.
    echo Похоже, сервер ещё не собран. Соберите его командой:
    echo     go build -o server\build\woodwerk-windows-amd64.exe .\server
    echo.
    pause
    exit /b 1
)

echo.
echo   WOODWERK
echo   ------------------------------------------
echo   Сайт:        http://127.0.0.1:8080/
echo   Админка:     http://127.0.0.1:8080/admin
echo.
echo   Это окно закрывать нельзя - в нём работает сервер.
echo   Чтобы остановить, нажмите Ctrl+C или закройте окно.
echo   ------------------------------------------
echo.

rem Браузер открываем с задержкой, чтобы сервер успел подняться.
start "" /b cmd /c "ping -n 3 127.0.0.1 >nul & start "" http://127.0.0.1:8080/admin"

"%EXE%" -addr :8080 -dir .

echo.
echo Сервер остановлен.
pause
