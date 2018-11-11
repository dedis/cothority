all: do-java do-js

do-java:
	@for i in proto/*.proto ; \
	do \
		echo $$i ; \
		protoc -I=proto --java_out=java/src/main/java $$i ; \
	done

	./clean_proto_deprecation.sh

do-js:
	cd js/cothority; npm run protobuf


