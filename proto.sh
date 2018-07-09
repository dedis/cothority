#!/bin/bash -e -u
struct_files=(`find . -name proto.go | sort`)

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
