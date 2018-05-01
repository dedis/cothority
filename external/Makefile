all: java js

java: proto/*
	protoc -I=proto --java_out=java/src/main/java proto/*proto

js: proto/*
	cd js/cothority; npm run protobuf
