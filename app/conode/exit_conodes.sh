#!/bin/bash

CONFIG=config.toml
if [ "$1" ]; then
  CONFIG=$1/$CONFIG
fi
HOSTS=$( grep Hosts $CONFIG | sed -e "s/.*\[\"\(.*\)\"\]/\1/" | perl -pe "s/\", \"/\n/g" | tail -r )

echo Going to ask all servers to exit
for h in $HOSTS; do
  # Suppose the last digit is 0 and we replace it by 1
  hp1=$( echo $h | sed -e "s/0\$/1/" )
  echo Asking server $hp1 to exit
  ./conode exit $hp1
done
