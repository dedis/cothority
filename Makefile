install:
	@export PR=https://api.github.com/repos/$$TRAVIS_REPO_SLUG/pulls/$$TRAVIS_PULL_REQUEST; \
	export BRANCH=$$(if [ "$$TRAVIS_PULL_REQUEST" == "false" ]; then echo $$TRAVIS_BRANCH; else echo `curl -s $$PR | jq -r .head.ref`; fi); \
	echo "TRAVIS_BRANCH=$$TRAVIS_BRANCH, PR=$$PR, BRANCH=$$BRANCH"; \
	pattern="refactor_"; \
	if [[ "$$BRANCH" =~ "$$pattern" ]]; then \
		repo=github.com/dedis/cosi; \
		go get $$repo; \
		cd $$GOPATH/src/$$repo; \
		git checkout -f $BRANCH; \
	fi;\
	cd $$GOPATH/src/github.com/dedis/cothority; \
	go get -t ./...

test_fmt:
	@echo Checking correct formatting of files
	@{ \
		files=$$( go fmt ./... ); \
		if [ -n "$$files" ]; then \
		echo "Files not properly formatted: $$files"; \
		exit 1; \
		fi; \
		if ! go vet ./...; then \
		exit 1; \
		fi \
	}

test_lint:
	@echo Checking linting of files
	@{ \
		go get github.com/golang/lint/golint; \
		exclude="protocols/byzcoin|_test.go"; \
		lintfiles=$$( golint ./... | egrep -v "($$exclude)" ); \
		if [ -n "$$lintfiles" ]; then \
		echo "Lint errors:"; \
		echo "$$lintfiles"; \
		exit 1; \
		fi \
	}

test_multi:
    cd network; \
	for a in $$( seq 10 ); do \
	  go test -v -race -run Stress; \
	done

test_verbose:
	go test -v -race -p=1 -short ./...

test_go:
	go test -race -p=1 -short ./...

test: test_fmt test_lint test_go

all: install test
