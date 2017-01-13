all: test

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
		go get -u github.com/golang/lint/golint; \
		exclude="byzcoin|_test.go"; \
		lintfiles=$$( golint ./... | egrep -v "($$exclude)" ); \
		if [ -n "$$lintfiles" ]; then \
		echo "Lint errors:"; \
		echo "$$lintfiles"; \
		exit 1; \
		fi \
	}

# You can use `test_playground` to run any test or part of cothority
# for more than once in Travis. Change `make test` in .travis.yml
# to `make test_playground`.
test_playground:
	( cd ../onet; git checkout debugging_cothority764 ); \
	cd identity; \
	for a in $$( seq 10 ); do \
	  go test -v -race -short || exit 1 ; \
	done;

test_verbose:
	go test -v -race -short ./...

# use test_verbose instead if you want to use this Makefile locally
test_go:
	./coveralls.sh ./cosi ./cisc ./byzcoin/*

test: test_fmt test_lint test_playground

