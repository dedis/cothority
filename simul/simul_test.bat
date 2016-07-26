if "%1"=="" (set DBG=1) else (set DBG=%1)
echo Building deploy-binary with debug-level: %DBG%
go build

for %%i in (runfiles\test*toml) do (
  echo Simulating %%~fi
  simul -debug %DBG% %%~fi
)