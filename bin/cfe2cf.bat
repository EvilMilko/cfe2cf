@echo off
:: Включаем поддержку кириллицы
chcp 65001 > nul
setlocal enabledelayedexpansion

:: 1. Проверяем, перетащили ли файл (используем правильное ==)
if "%~1" == "" (
    echo [Ошибка] Вы просто кликнули на батник. 
    echo Чтобы он сработал, перетащите на него файл .cfe мышкой!
    echo.
    pause
    exit /b
)

:: 2. Проверяем, существует ли файл
if not exist "%~1" (
    echo [Ошибка] Указанный файл не найден.
    pause
    exit /b
)

:: 3. Проверяем, что это файл, а не папка
dir /a-d "%~1" >nul 2>&1
if errorlevel 1 (
    echo [Ошибка] Вы перетащили папку. Нужен именно файл .cfe!
    pause
    exit /b
)

:: 4. Извлекаем имя файла без расширения
set "fullname=%~n1"
set "ext_name=!fullname!"

:: 5. Проверяем наличие даты _YYYYMMDD на конце (9 символов, первый из которых "_")
set "suffix=!fullname:~-9!"
if "!suffix:~0,1!" == "_" (
    set "digits=!suffix:~1!"
    :: Проверяем, что в хвосте только цифры
    for /f "delims=0123456789" %%a in ("!digits!") do set "not_digits=%%a"
    if not defined not_digits (
        set "ext_name=!fullname:~0,-9!"
    )
)

:: 6. Проверяем наличие cfe2cf.exe рядом с батником
if not exist "%~dp0cfe2cf.exe" (
    echo [Ошибка] В папке батника не найден файл cfe2cf.exe!
    echo Он должен лежать здесь: "%~dp0"
    echo.
    pause
    exit /b
)

:: 7. Запуск конвертации
echo Файл источника:  "%~nx1"
echo Имя расширения:  "!ext_name!"
echo Выходной файл:   "%~n1.cf"
echo ------------------------------------------------------------

"%~dp0cfe2cf.exe" f "%~1" "!ext_name!" "%~dp0%~n1.cf"

echo ------------------------------------------------------------
echo Готово!
pause