#!/usr/bin/env bash
# Source: https://github.com/h12w/gosweep/blob/master/gosweep.sh

DIR_SOURCE="$(find . -maxdepth 10 -type f -not -path '*/vendor*' -name '*.go' | xargs -I {} dirname {} | sort | uniq)"

# Run test coverage on each subdirectories and merge the coverage profile.

echo "mode: atomic" > profile.cov

for dir in ${DIR_SOURCE};
do
    go test -short -race -covermode=atomic -coverprofile=$dir/profile.tmp $dir
    if [ -f $dir/profile.tmp ]
    then
        cat $dir/profile.tmp | tail -n +2 >> profile.cov
        rm $dir/profile.tmp
    fi
done

# If you want to print the coverage of each file individually uncomment:
# go tool cover -func profile.cov

# To submit the test coverage result to coveralls.io,
# use goveralls (https://github.com/mattn/goveralls)
goveralls -coverprofile=profile.cov -service=travis-ci -repotoken $COVERALLS_TOKEN
