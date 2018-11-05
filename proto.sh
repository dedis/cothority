#!/usr/bin/env bash

set -e
set -u

struct_files=(`find . -name proto.go | sort`)

pv=`protoc --version`
if [ "$pv" != "libprotoc 3.6.1" ]; then
	echo "Protoc version $pv is not supported."
	exit 1
fi

for index in ${!struct_files[@]}; do
  filename=${struct_files[index]}
  ret=$( grep "// package" "$filename" | sed -e "s/.* //" | sed -e "s/;//" ).proto
  if [ "$ret" = ".proto" ]; then
    echo "Please add package name to $filename"
    exit 1
  fi
  ret=external/proto/$ret
  echo "$filename => $ret"
  mkdir -p "$(dirname "$ret")" && awk -f proto.awk "$filename" > "$ret"
done
