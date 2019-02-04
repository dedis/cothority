all: do-java do-js

do-java:
	@for i in proto/*.proto ; \
	do \
		echo $$i ; \
		protoc -I=proto --java_out=java/src/main/java $$i ; \
	done

	./clean_proto_deprecation.sh

do-js:
	@test -d js/cothority/node_modules/protobufjs || (echo "Error: Must run \"npm install\" in external/js/cothority first."; exit 1)
	cd js/cothority; npm run protobuf


