#!/bin/bash

# Because Protobuf uses its own deprecated references, this script will replace
# them by the correct public getter

echo "Fixing PARSER deprecation by using the public getter"

PROTO_PATH=java/src/main/java/ch/epfl/dedis/lib/proto

for filename in $PROTO_PATH/*.java; do
	sed -i.bak 's/\.PARSER/.parser()/g' $filename
	rm ${filename}.bak
done
