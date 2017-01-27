#!/usr/bin/env bash
cd static
if ! which bower; then
	echo "bower is not installed - go have a look at https://bower.io/"
	exit 1
fi
bower install bootstrap jquery qcode-decoder
mkdir -p gopherjs
cd gopherjs
gopherjs build ../sig.go
cd ../..
go build
if [ ! -f server.key ]; then
	./cert.sh
fi
echo "All is set up - will run website now."
./33c3 ../final.toml schedule.json