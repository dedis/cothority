all: do-java do-js

do-java:
	@for i in proto/*.proto ; \
	do \
		echo $$i ; \
		protoc -I=proto --java_out=java/src/main/java $$i ; \
	done

do-js:
	cd js/cothority; npm run protobuf


