#!/usr/bin/env bash
set -e

NBR=3
killall cothorityd || true
go build ../../app/cothorityd

for n in $( seq $NBR ); do
	co=co$n
	if ! grep -q Description $co/config.toml; then
		echo "Detected old files - deleting"
		rm -rf $co
	fi

	if [ ! -d $co ]; then
		echo -e "$((7000 + $n))\n\nConode $n\n$co" | ./cothorityd setup
	fi
	./cothorityd -c $co/config.toml -d 3 &
done

grep -vh Description co*/group.toml > group.toml

go build github.com/dedis/cosi
echo "Everything is set up - if you want to make some traffic, type"
echo "./cosi check group.toml"