CONTAINER = conode
IMAGE_NAME = dedis/$(CONTAINER)
VERSION = 1.1.1-2

all: docker link

docker: Dockerfile
	docker build -t $(IMAGE_NAME):$(VERSION) -t $(IMAGE_NAME):latest ./

docker_run:
	mkdir -p data
	docker run -it --rm -p 6879-6880:6879-6880 --name $(CONTAINER) -v $$(pwd)/conode_data:/root/.local/share/conode/ \
	    -v $$(pwd)/conode_data:/root/.config//conode $(IMAGE_NAME)

clean:
	docker kill $(CONTAINER) || echo nothing to stop
	docker image ls $(IMAGE_NAME) -q | xargs docker rmi -f || echo done
