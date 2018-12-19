all: test

# gopkg fits all v1.1, v1.2, ... in v1
PKG_STABLE = gopkg.in/dedis/cothority.v2
include $(shell go env GOPATH)/src/github.com/dedis/Coding/bin/Makefile.base
EXCLUDE_LINT = "should be.*UI|_test.go"

# You can use `test_playground` to run any test or part of cothority
# for more than once in Travis. Change `make test` in .travis.yml
# to `make test_playground`.
test_playground:
	cd byzcoin; \
	for a in $$( seq 100 ); do \
		if DEBUG_TIME=true go test -v -race > log.txt 2>&1; then \
			echo Successfully ran \#$$a at $$(date); \
		else \
			echo Failed at $$(date); \
			cat log.txt; \
			exit 1; \
		fi; \
	done;

# Other targets are:
# make create_stable

proto:
	./proto.sh
	make -C external


docker:
	cd conode/; make docker_dev
	cd external/docker/; make docker_test

test_java: docker
	cd external/java; mvn test

test_nostable: test_fmt test_lint test_goveralls
