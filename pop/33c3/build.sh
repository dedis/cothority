#!/usr/bin/env bash
cd static
cd js
gopherjs build ../sig.go
cd ../..
go build
if [ ! -f server.key ]; then
	./cert.sh
fi
echo "All is set up - will run website now."
./33c3 final.toml schedule.json
