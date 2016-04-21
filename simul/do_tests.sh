#!/usr/bin/env bash 

DBG=${1:-1}
echo Building deploy-binary with debug-level: $DBG
go build

for rf in runfiles/test*toml; do
  echo Simulating $rf
  ./simul -debug $DBG $rf
  echo -e "\n\n"
done
