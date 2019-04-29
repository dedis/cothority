all: run_conodes run_main

run_main:
	go run main.go data/public.toml

run_conodes: compile_conode stop_conodes
	docker run -p 7770-7775:7770-7775 --rm -v$$(pwd)/data:/conode_data -e COTHORITY_ALLOW_INSECURE_ADMIN=true \
		--name conode_test c4dt/ocs:dev ./run_nodes.sh -n 3 -v 2 -c -d /conode_data &
	sleep 5

compile_conode:
	make -C ../../conode docker_dev

stop_conodes:
	docker rm -f conode_test || true
