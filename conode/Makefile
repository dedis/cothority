CONTAINER = conode
IMAGE_NAME = dedis/$(CONTAINER)
GITCOMMIT := $(shell git rev-parse --short HEAD)
GITUNTRACKEDCHANGES := $(shell git status --porcelain --untracked-files=no)
ifneq ($(GITUNTRACKEDCHANGES),)
	GITCOMMIT := $(GITCOMMIT)-dirty
endif
VERSION = $(GITCOMMIT)
DATA_DIR = $(shell pwd)/conode_data

all: docker

docker: Dockerfile
	[ -f $(HOME)/.netrc ] && cp $(HOME)/.netrc .
	docker build -t $(IMAGE_NAME):$(VERSION) .
	rm -f .netrc

push:
	@[ -n "$(GITUNTRACKEDCHANGES)" ] && echo "Pushing dirty images not allowed." && exit 1 || true
	docker push $(IMAGE_NAME):$(VERSION)

docker_setup:
	mkdir -p $(DATA_DIR)
	docker run -it --rm -P --name $(CONTAINER) -v $(DATA_DIR):/conode_data \
	    $(IMAGE_NAME) ./conode setup

docker_run:
	if [ ! -f conode_data/private.toml ]; then make docker_setup; fi
	mkdir -p $(DATA_DIR)
	docker run -it --rm -P --name $(CONTAINER) -v $(DATA_DIR):/conode_data \
	    $(IMAGE_NAME)

docker_stop:
	docker rm -f $(CONTAINER)

docker_clean:
	docker kill $(CONTAINER) || echo nothing to stop
	docker image ls $(IMAGE_NAME) -q | xargs docker rmi -f || echo done

TAG=$(shell git describe --tags)
OUTPUT_DIR=conode-$(TAG)
# -s -w are for smaller binaries
# -X compiles the git tag into the binary
flags=-ldflags="-s -w -X main.gitTag=$(TAG) -X github.com/dedis/onet.gitTag=$(TAG)"

# The suffix on conode exe is the result from: echo `uname -s`.`uname -m`
# so that we can find the right one in the wrapper script.
bindist:
	vgo verify
	rm -rf $(OUTPUT_DIR)
	mkdir $(OUTPUT_DIR)
	GOOS=linux GOARCH=amd64 vgo build $(flags) -o $(OUTPUT_DIR)/conode.Linux.x86_64
	GOOS=linux  GOARCH=arm vgo build $(flags) -o $(OUTPUT_DIR)/conode.Linux.armv7l
	GOOS=freebsd GOARCH=amd64 vgo build $(flags) -o $(OUTPUT_DIR)/conode.FreeBSD.amd64
	GOOS=windows GOARCH=386 vgo build $(flags) -o $(OUTPUT_DIR)/conode.exe
	echo "#!/bin/sh" > $(OUTPUT_DIR)/conode
	echo "./conode.\`uname -s\`.\`uname -m\` \$$*" >> $(OUTPUT_DIR)/conode
	chmod +x $(OUTPUT_DIR)/conode	
	LANG=C tar zcf $(OUTPUT_DIR).tar.gz $(OUTPUT_DIR)
	rm -rf $(OUTPUT_DIR)

gitTag:
	go build $(flags)
