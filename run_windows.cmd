@echo off
setlocal

pushd "%~dp0" || exit /b 1

where py >nul 2>nul
if %ERRORLEVEL%==0 (
    py -3 scripts\run_windows.py %*
    set "EXIT_CODE=%ERRORLEVEL%"
) else (
    where python >nul 2>nul
    if %ERRORLEVEL%==0 (
        python scripts\run_windows.py %*
        set "EXIT_CODE=%ERRORLEVEL%"
    ) else (
        echo ERROR: Python was not found. Install Python or add it to PATH.
        set "EXIT_CODE=1"
    )
)

popd

if not "%EXIT_CODE%"=="0" (
    echo.
    echo Petri Dish failed to launch. Exit code: %EXIT_CODE%
    pause
)

exit /b %EXIT_CODE%
