#!/usr/bin/env bash

set -e

echo "Running tests with tag '$1'"

export DEBUG_TIME=true

for d in $( find . -name "*go" | xargs -n 1 dirname | sort -u ); do
	# Do each directory on its own, but exclude if 'experimental' is found in
	# the first line.
	if sed -n 1p $d/*.go | grep -q -v experimental; then
		echo "Testing directory $d at $(date)"
		go test -tags $1 -p=1 -count=1 -v -race -timeout 30m $d || exit 1
	fi
done
