all: java

java: proto/*
	protoc -I=proto --java_out=java/ch.epfl.dedis.ocs/src/main/java proto/*proto
