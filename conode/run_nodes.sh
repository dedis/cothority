#!/usr/bin/env bash
set -e

NBR=${1:-3}
DEBUG=${2:-0}

rm -f public.toml
for n in $( seq $NBR ); do
  co=co$n
  if [ ! -d $co ]; then
    echo -e "localhost:$(($PORTBASE + 2 * $n))\nConode_$n\n$co" | conode setup
  fi
  conode -d $DEBUG -c $co/private.toml server &
  cat $co/public.toml >> public.toml
done
sleep 1

while true; do
  sleep 1;
done
