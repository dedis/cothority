#!/usr/bin/env bash
# script checks for missing comments using golint

# list of lints we want to ensure:
failOn="comment on exported type\|should have comment or be unexported\| by other packages, and that stutters; consider calling this"
packagesIgnore="protocols/byzcoin/blockchain|protocols/jvss"
lintOut=`$GOPATH/bin/golint ./... | egrep -v "($packagesIgnore)" |  grep "$failOn"`

# if the output isn't empty exit with an error
if [ -z "$lintOut" ]; then
  exit 0;
else
  echo "--------------------------------------------------------"
  echo "Lint failed:";
  echo "$lintOut";
  exit 1;
fi
