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
	which protoc || (echo "Install protoc in version 3.6.1. On Mac, run \"brew install protobuf@3.6\"" && exit 1)
	@pv=`protoc --version`; \
	if [ "$$pv" != "libprotoc 3.6.1" ]; then \
		echo "Protoc version $$pv is not supported."; \
		exit 1; \
	fi
