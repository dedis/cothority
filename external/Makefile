all: java js

java:
	protoc -I=proto --java_out=java/src/main/java proto/*proto

js:
	cd js/cothority; npm run protobuf
