#!/usr/bin/env bash

EXCLUDE="$@"
DIRS="$(find . -maxdepth 10 -type f -name '*.go' | xargs -I {} dirname {} | sort | uniq)"

passed=true

echo "mode: atomic" > profile.cov
for dir in $DIRS; do
	if ! echo $EXCLUDE | grep -q $dir; then
	    go test -short -covermode=atomic -coverprofile=$dir/profile.tmp $dir

    	if [ $? -ne 0 ]; then
        	passed=false
    	fi
    	if [ -f $dir/profile.tmp ]; then
         	tail -n +2 $dir/profile.tmp >> profile.cov
        	rm $dir/profile.tmp
    	fi
    fi
done

if [ "$passed" = true ]; then
    exit 0
else
    exit 1
fi
