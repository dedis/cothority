#!/usr/bin/env bash
# script checks for forgotten/missing error handling
# make sure you installed errcheck first:
# go get -u github.com/kisielk/errcheck

# list of lints we want to ensure:
Ignore="_test.go"
Out=`$GOPATH/bin/errcheck ./... | grep -v "$Ignore"`

# if the output isn't empty exit with an error
if [ -z "$Out" ]; then
  exit 0;
else
  echo "--------------------------------------------------------"
  echo "Error handling is missing:";
  echo "$Out";
  exit 1;
fi
