all: do-java do-js

do-java: check_protoc
	@for i in proto/*.proto ; \
	do \
		echo $$i ; \
		protoc -I=proto --java_out=java/src/main/java $$i ; \
	done

	./clean_proto_deprecation.sh

do-js: check_protoc
	@test -d js/cothority/node_modules/protobufjs || (echo "Error: Must run \"npm install\" in external/js/cothority first."; exit 1)
	cd js/cothority; npm run protobuf


check_protoc:
	which protoc || $(MAKE) protoc_instructions
	pv=`protoc --version`; \
	if [ "$$pv" != "libprotoc 3.17.3" ]; then \
		echo "Protoc version $$pv is not supported."; \
		$(MAKE) protoc_instructions; \
	fi

protoc_instructions:
	echo "Please download version 3.17.3 from here:"
	echo "https://github.com/protocolbuffers/protobuf/releases/tag/v3.17.3"
	exit 1
