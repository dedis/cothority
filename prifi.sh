#!/bin/bash
lvl=3
if [ $# -eq 1 ]
  then
    lvl=$1
fi
echo "Running PriFi simulation through SDA, debug level is $lvl, output is in log.txt"
cd simul;
go build
./simul -debug $lvl runfiles/prifi_simple.toml -platform localhost | tee ../log.txt
