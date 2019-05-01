# This docker container is used from Jenkins in order to run the unit
# tests such that they do not conflict with other things running on the
# same server.

# The container image is built as follows:
#
#   docker build -f Dockerfile-unit -t dedis/cothority-unit-tester --build-arg test_tag=(test_long|)
#
# As you can see, you can run the long tests using the tag build argument.

FROM golang:1.12

ARG test_tag
ENV TEST_TAG=$test_tag

WORKDIR /cothority
CMD ./unit_test.sh ${TEST_TAG}
