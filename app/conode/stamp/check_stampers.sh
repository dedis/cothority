#!/bin/bash

HOSTS=$( cat config.toml | grep Hosts | sed -e "s/.*\[\"\(.*\)\"\]/\1/" | perl -pe "s/\", \"/\n/g" | \
sed -e "s/:2000//" )

echo Going to stamp from servers
for h in $HOSTS; do
  echo Stamping using server $h
  ./stamp sign stamp -server $h
done
