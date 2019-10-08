all: test

include $(shell go env GOPATH)/src/github.com/dedis/Coding/bin/Makefile.base
EXCLUDE_LINT = "should be.*UI|_test.go"

# You can use `test_playground` to run any test or part of cothority
# for more than once in Travis. Change `make test` in .travis.yml
# to `make test_playground`.
test_playground:
	for a in $$( seq 100 ); do \
    	cd byzcoin/contracts; \
		if DEBUG_TIME=true go test -v -race -run TestDeferred_WrongSignature > \
		        log.txt 2>&1; then \
			echo Successfully ran \#$$a at $$(date); \
		else \
			echo Failed at $$(date); \
			cat log.txt; \
			exit 1; \
		fi; \
    	cd ../../skipchain; \
		if DEBUG_TIME=true go test -v -race -run TestService_MissingForwardlink > \
		        log.txt 2>&1; then \
			echo Successfully ran \#$$a at $$(date); \
		else \
			echo Failed at $$(date); \
			cat log.txt; \
			exit 1; \
		fi; \
		cd ..; \
	done;

proto:
	./proto.sh
	make -C external

docker:
	cd conode/; make docker_dev
	cd external/docker/; make docker_test

test_java: docker
	cd external/java; mvn test

test_proto:
	@./proto.sh > /dev/null; \
	if [ "$$( git diff )" ]; then \
		echo "Please update proto-files with 'make proto'"; \
		exit 1; \
	fi