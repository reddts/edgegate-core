@echo off
setlocal
set PATH=D:\msys64\mingw64\bin;D:\msys64\usr\bin;%PATH%
set GOOS=windows
set GOARCH=amd64
set CC=x86_64-w64-mingw32-gcc
set CXX=x86_64-w64-mingw32-g++
set CGO_ENABLED=1
set EDGEGATE_TAGS=with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api

go run ./cmd/main tunnel exit

if exist bin\edgegate-core.dll del /f /q bin\edgegate-core.dll
if exist bin\EdgegateCli.exe del /f /q bin\EdgegateCli.exe

set CGO_LDFLAGS=
go build -trimpath -tags %EDGEGATE_TAGS% -ldflags="-w -s" -buildmode=c-shared -o bin\edgegate-core.dll ./platform/desktop
if errorlevel 1 exit /b %errorlevel%
copy /y bin\edgegate-core.dll bin\libcore.dll >nul
copy /y bin\edgegate-core.h bin\libcore.h >nul

go install -mod=readonly github.com/akavel/rsrc@latest
if errorlevel 1 (
  echo [warn] failed to install rsrc, skipping EdgegateCli.exe build
  goto :done
)

for /f %%i in ('go env GOPATH') do set EDGEGATE_GOPATH=%%i
"%EDGEGATE_GOPATH%\bin\rsrc.exe" -ico .\assets\edgegate-cli.ico -o cmd\bydll\cli.syso
if errorlevel 1 (
  echo [warn] failed to generate CLI resources, skipping EdgegateCli.exe build
  goto :done
)

copy /y bin\edgegate-core.dll edgegate-core.dll >nul
set CGO_LDFLAGS=edgegate-core.dll
go build -trimpath -tags %EDGEGATE_TAGS% -ldflags="-s -w" -o bin\EdgegateCli.exe ./cmd/bydll/
set BUILD_EXIT=%ERRORLEVEL%
if exist edgegate-core.dll del /f /q edgegate-core.dll
if not "%BUILD_EXIT%"=="0" exit /b %BUILD_EXIT%

:done
endlocal
