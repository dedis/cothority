#!/usr/bin/env bash
# Source: https://github.com/h12w/gosweep/blob/master/gosweep.sh

DIR_SOURCE="$(find . -maxdepth 10 -type f -not -path '*/vendor*' -name '*.go' | xargs -I {} dirname {} | sort | uniq)"


BRANCH=$TRAVIS_PULL_REQUEST_BRANCH
echo "Using branch $BRANCH"

pattern="refactor_*"
if [[ $BRANCH =~ $pattern ]]; then 
    echo "Using refactored branch $BRANCH - fetching cosi"
    repo=github.com/dedis/cosi
    go get -d $repo
    cd $GOPATH/src/$repo
    git checkout -f $BRANCH
    echo $(git rev-parse --abbrev-ref HEAD)
fi

if [ "$TRAVIS_BUILD_DIR" ]; then
  cd $TRAVIS_BUILD_DIR
fi

# Run test coverage on each subdirectories and merge the coverage profile.
all_tests_passed=true

echo "mode: atomic" > profile.cov
for dir in ${DIR_SOURCE};
do
    go test -short -race -covermode=atomic -coverprofile=$dir/profile.tmp $dir
    if [ $? -ne 0 ]; then
        all_tests_passed=false
    fi
    if [ -f $dir/profile.tmp ]
    then
        cat $dir/profile.tmp | tail -n +2 >> profile.cov
        rm $dir/profile.tmp
    fi
done

if [[ $all_tests_passed = true ]];
then
    exit 0;
else
    exit 1;
fi
