#!/bin/bash

struct_files=(`find . -name proto.go | sort`)

for index in ${!struct_files[@]}; do
    filename=${struct_files[index]}
    ret=$(cut -c 3- <<<$filename)
    ret=$(echo "${ret/\/proto.go/.proto}")
    ret=external/proto/$ret
    echo $filename ==} $ret
    mkdir -p "$(dirname "$ret")" && awk -f proto.awk $filename > $ret
done