all: java

java: proto/*
	protoc -I=proto --java_out=java/src/main/java proto/*proto
