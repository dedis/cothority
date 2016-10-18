#!/usr/bin/env bash
set -e

NBR=3
killall cothorityd || true
go build ../../app/cothorityd

for n in $( seq $NBR ); do
	if [ ! -d co$n ]; then
		echo -e "$((7000 + $n))\n\nco$n" | ./cothorityd setup
	fi
	./cothorityd -c co$n/config.toml -d 3 &
done