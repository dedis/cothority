#!/usr/bin/env bash

docker run --rm -v ${1}:/project -w /project whispersystems/signal-android:0.2 ./gradlew clean assembleRelease > /dev/null 2>&1
# Return location of a binary
printf "build/outputs/apk/project-release-unsigned.apk"