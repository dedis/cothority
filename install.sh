#!/bin/bash

## install.sh will fetch all dependencies of the project.
## If the branch from where the PR comes from starts with
## `refactor_`, it will checkout the same branch name
## in dedis/cosi.


if [ "$TRAVIS_PULL_REQUEST" = "false" ]; then
  BRANCH=$TRAVIS_BRANCH
else
  # source: https://gist.github.com/derekstavis/0526ac13cfecb5d6ffe5#file-travis-github-pull-request-integration-sh
  GITHUB_PR_URL=https://api.github.com/repos/$TRAVIS_REPO_SLUG/pulls/$TRAVIS_PULL_REQUEST
  GITHUB_PR_BODY=$(curl -s $GITHUB_PR_URL 2>/dev/null)

  if [[ $GITHUB_PR_BODY =~ \"ref\":\ *\"([a-zA-Z0-9_-]*)\" ]]; then
    export TRAVIS_BRANCH=${BASH_REMATCH[1]}
  fi

  if [[ $GITHUB_PR_BODY =~ \"repo\":.*\"clone_url\":\ *\"https://github\.com/([a-zA-Z0-9_-]*/[a-zA-Z0-9_-]*)\.git.*\"base\" ]]; then
    export TRAVIS_REPO_SLUG=${BASH_REMATCH[1]}
  fi
fi

# If you don't believe in travis-magic:
#BRANCH=refactor_cothority_506
#BRANCH=$TRAVIS_BRANCH
echo "Thinking we're on branch $BRANCH"

pattern="refactor_*";
if [[ $BRANCH =~ $pattern ]]; then 
    echo "Using refactored branch $BRANCH - fetching cosi"
    repo=github.com/dedis/cosi; 
    go get -d $repo; 
    cd $GOPATH/src/$repo; 
    git checkout -f $BRANCH; 
    echo $(git rev-parse --abbrev-ref HEAD);
fi;

cd $GOPATH/src/github.com/dedis/cothority; 
go get -t ./...
