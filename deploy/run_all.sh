#!/usr/bin/env bash -e

echo Building deploy-binary
go build

for simul in simulation/test*toml; do
  echo Simulating $simul
  ./deploy $simul
  echo -e "\n\n"
done