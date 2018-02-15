all: test

# gopkg fits all v1.1, v1.2, ... in v1
PKG_STABLE = gopkg.in/dedis/cothority.v2
include $(shell go env GOPATH)/src/github.com/dedis/Coding/bin/Makefile.base

# You can use `test_playground` to run any test or part of cothority
# for more than once in Travis. Change `make test` in .travis.yml
# to `make test_playground`.
test_playground:
	cd skipchain; \
	for a in $$( seq 100 ); do \
	  go test -race -short || exit 1 ; \
	done;

# Other targets are:
# make create_stable

proto:
	awk -f proto.awk status/service/struct.go > external/proto/status.proto
