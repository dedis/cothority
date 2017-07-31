IMAGE_NAME = dedis/onchain-secrets

docker:
	docker build -t $(IMAGE_NAME) .

docker_run:
	docker run -it --rm -p 7003:7003 -p 7005:7005 -p 7007:7007 --name ocs \
	 -v ./data:/root/.local/share/conode $(IMAGE_NAME)

