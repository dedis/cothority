#!/bin/bash

HOSTS=$( cat config.toml | grep Hosts | sed -e "s/.*\[\"\(.*\)\"\]/\1/" | perl -pe "s/\", \"/\n/g" )

echo Going to ask all servers to exit
for h in $HOSTS; do
  # Suppose the last digit is 0 and we replace it by 1
  hp1=$( echo $h | sed -e "s/0\$/1/" )
  echo Asking server $hp1 to exit
  ./conode exit $hp1
done
