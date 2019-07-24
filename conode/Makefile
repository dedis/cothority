CONTAINER = conode
IMAGE_NAME = dedis/$(CONTAINER)
DATA_DIR = $(shell pwd)/conode_data
GITUNTRACKEDCHANGES := $(shell git status --porcelain --untracked-files=no)
TAG = dev-$(shell date +%y%m%d)
GIT_TAG = $(shell git tag -l --points-at HEAD )
ifneq ($(GIT_TAG),)
  TAG = $(GIT_TAG)
endif
OUTPUT_DIR = conode-$(TAG)

# -s -w are for smaller binaries
# -X compiles the git tag into the binary
ldflags=-s -w -X main.gitTag=$(TAG)

all: docker

.PHONY: docker-base
docker-base: Dockerfile-base
	docker build -t $(IMAGE_NAME)-base:latest -f Dockerfile-base .

# Use this target to build from only published sources.
docker: clean Dockerfile docker-base
	@[ -z "$(BUILD_TAG)" ] && echo "Must specify BUILD_TAG." && exit 1 || true
	docker build -t $(IMAGE_NAME):$(BUILD_TAG) \
		--build-arg BUILD_TAG="$(BUILD_TAG)" --build-arg ldflags="$(ldflags)" \
		--build-arg FROM=$(IMAGE_NAME)-base:latest .
	docker tag $(IMAGE_NAME):$(BUILD_TAG) $(IMAGE_NAME):dev

# Use this target to build from local source instead of from publish sources.
docker_dev: clean Dockerfile-dev verify exe/conode.Linux.x86_64 docker-base
	docker build -t $(IMAGE_NAME):$(TAG) -f Dockerfile-dev --build-arg FROM=$(IMAGE_NAME)-base:latest .
	docker tag $(IMAGE_NAME):$(TAG) $(IMAGE_NAME):dev

docker_push: docker
	@[ -n "$(GITUNTRACKEDCHANGES)" ] && echo "Pushing dirty images not allowed." && exit 1 || true
	docker push $(IMAGE_NAME):$(BUILD_TAG)

docker_push_latest: docker_push
	docker tag $(IMAGE_NAME):dev $(IMAGE_NAME):latest

docker_setup:
	mkdir -p $(DATA_DIR)
	docker run -it --rm -p 7770-7771:7770-7771 --name $(CONTAINER) -v $(DATA_DIR):/conode_data \
	    $(IMAGE_NAME):dev ./conode setup

docker_run:
	if [ ! -f conode_data/private.toml ]; then make docker_setup; fi
	mkdir -p $(DATA_DIR)
	docker run -it --rm -p 7770-7771:7770-7771 --name $(CONTAINER) -v $(DATA_DIR):/conode_data \
	    $(IMAGE_NAME):dev

docker_stop:
	docker rm -f $(CONTAINER)

docker_clean:
	docker kill $(CONTAINER) || echo nothing to stop
	docker image ls $(IMAGE_NAME) -q | xargs docker rmi -f || echo done

clean:
	rm -rf exe $(OUTPUT_DIR)

verify:
	GO111MODULE=on go mod verify
	@echo "Checking for replace in go.mod..."
	@if GO111MODULE=on go list -m all | grep --quiet '=>'; then exit 1; fi

# The suffix on conode exe is the result from: echo `uname -s`.`uname -m`
# so that we can find the right one in the wrapper script.
# This is in it's own rule because the Docker build needs it also.
exe/conode.Linux.x86_64:
	GO111MODULE=on GOOS=linux GOARCH=amd64 go build -ldflags="$(ldflags)" -o $@

bindist: clean verify exe/conode.Linux.x86_64
	rm -rf $(OUTPUT_DIR)
	mkdir $(OUTPUT_DIR)
	cp exe/conode.Linux.x86_64 $(OUTPUT_DIR)
	GO111MODULE=on GOOS=darwin GOARCH=amd64 go build -ldflags="$(ldflags)" -o $(OUTPUT_DIR)/conode.Darwin.x86_64
	GO111MODULE=on GOOS=freebsd GOARCH=amd64 go build -ldflags="$(ldflags)" -o $(OUTPUT_DIR)/conode.FreeBSD.amd64
	GO111MODULE=on GOOS=windows GOARCH=amd64 go build -ldflags="$(ldflags)" -o $(OUTPUT_DIR)/conode.exe
	echo "#!/bin/sh" > $(OUTPUT_DIR)/conode
	echo "./conode.\`uname -s\`.\`uname -m\` \$$*" >> $(OUTPUT_DIR)/conode
	chmod +x $(OUTPUT_DIR)/conode
	LANG=C tar zcf $(OUTPUT_DIR).tar.gz $(OUTPUT_DIR)
	rm -rf $(OUTPUT_DIR)
	ls -l $(OUTPUT_DIR).tar.gz

tooldist: clean verify
	rm -rf conode-tools-$(TAG)
	mkdir conode-tools-$(TAG)
	GO111MODULE=on GOOS=linux GOARCH=amd64 go build -ldflags="$(ldflags)" -o conode-tools-$(TAG)/bcadmin ../byzcoin/bcadmin
	GO111MODULE=on GOOS=linux GOARCH=amd64 go build -ldflags="$(ldflags)" -o conode-tools-$(TAG)/status ../status
	GO111MODULE=on GOOS=linux GOARCH=amd64 go build -ldflags="$(ldflags)" -o conode-tools-$(TAG)/scmgr ../scmgr
	GO111MODULE=on GOOS=linux GOARCH=amd64 go build -ldflags="$(ldflags)" -o conode-tools-$(TAG)/evoting-admin ../evoting/evoting-admin
	GO111MODULE=on GOOS=linux GOARCH=amd64 go build -ldflags="$(ldflags)" -o conode-tools-$(TAG)/csadmin ../calypso/csadmin
	LANG=C tar zcf conode-tools-$(TAG).tar.gz conode-tools-$(TAG)
	rm -rf conode-tools-$(TAG)
	ls -l conode-tools-$(TAG).tar.gz
