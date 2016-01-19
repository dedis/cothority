#!/usr/bin/env bash -e

DBG=${1:-1}
echo Building deploy-binary with debug-level: $DBG
go build

for simul in simulation/test*toml; do
  echo Simulating $simul
  ./deploy -debug $DBG $simul
  echo -e "\n\n"
done
