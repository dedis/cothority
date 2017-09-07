CONTAINER = conode
IMAGE_NAME = dedis/$(CONTAINER)
VERSION = 1.1.1-2
DATA_DIR = $(shell pwd)/conode_data

all: docker

docker: Dockerfile
	docker build -t $(IMAGE_NAME):$(VERSION) -t $(IMAGE_NAME):latest ./

docker_run:
	mkdir -p $(DATA_DIR)
	docker run -it --rm -p 6879-6880:6879-6880 --name $(CONTAINER) -v $(DATA_DIR):/root/.local/share/conode/ \
	    -v $(DATA_DIR):/root/.config/conode $(IMAGE_NAME)

docker_stop:
	docker rm -f $(CONTAINER)

docker_clean:
	docker kill $(CONTAINER) || echo nothing to stop
	docker image ls $(IMAGE_NAME) -q | xargs docker rmi -f || echo done
