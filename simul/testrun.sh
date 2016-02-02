#!/usr/bin/env bash -e

echo Building deploy-binary
go build

for rc in runconfig/test*toml; do
  echo Simulating $rc
  ./simul $rc
  echo -e "\n\n"
done
