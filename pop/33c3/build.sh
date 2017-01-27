#!/usr/bin/env bash
( cd static/js; gopherjs build sig.go )
go build
if [ ! -f server.key ]; then
	./cert.sh
fi
echo "All is set up - will run website now."
./33c3 final.toml schedule.json
