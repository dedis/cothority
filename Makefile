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

# You can use `test_multi` to run any test or part of cothority
# for more than once in Travis. Change `make test` in .travis.yml
# to `make test_multi`.
test_multi:
	cd services/identity; \
	for a in $$( seq 100 ); do \
	  go test -v -race -run ConfigNewCheck || exit 1 ; \
	done; \

test_verbose:
	go test -v -race -short ./...

test_go:
	go test -race -short ./...

test: test_fmt test_lint test_go

all: install test
