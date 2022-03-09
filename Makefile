.DEFAULT_GOAL := test

EXCLUDE_LINT := should be.*UI

TEST_IMAGE_NAME = dedis/conode-test

Coding/bin/Makefile.base:
	git clone https://github.com/dedis/Coding
include Coding/bin/Makefile.base

# You can use `test_playground` to run any test or part of cothority
# for more than once in Travis. Change `make test` in .travis.yml
# to `make test_playground`.
test_playground:
	cd personhood; \
	for a in $$( seq 100 ); do \
		if DEBUG_TIME=true go test -v -race > log.txt 2>&1; then \
			echo Successfully ran \#$$a at $$(date); \
		else \
			echo Failed at $$(date); \
			cat log.txt; \
			exit 1; \
		fi; \
	done;

proto:
	./proto.sh
	make -C external

docker:
	docker run -t -v $(PWD):/cothority golang:1.15-buster \
		bash -c "cd /cothority && \
		go build -o external/docker/conode -tags test ./conode && \
		go build -o external/docker/csadmin -tags test ./calypso/csadmin && \
		go build -o external/docker/bcadmin -tags test ./byzcoin/bcadmin"
	cp conode/run_nodes.sh external/docker
	docker build -t $(TEST_IMAGE_NAME) external/docker

docker_test_run: docker
	docker run -ti -p7770-7777:7770-7777 $(TEST_IMAGE_NAME)

test_java: docker
	cd external/java; mvn test

test_proto:
	@./proto.sh > /dev/null; \
	if [ "$$( git diff )" ]; then \
		echo "Please update proto-files with 'make proto'"; \
		exit 1; \
	fi

test_llvl:
	@if find . -name "*.go" | xargs grep -r log.LLvl; then \
		echo "LLvl is only for debugging"; exit 1; \
	fi
