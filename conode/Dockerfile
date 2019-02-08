FROM golang:1.11 as builder
ARG BUILD_TAG=none
ARG ldflags="-s -w -X main.gitTag=unknown"
RUN go get github.com/dedis/cothority
RUN cd /go/src/github.com/dedis/cothority/conode && git checkout $BUILD_TAG && GO111MODULE=on go install -ldflags="$ldflags" .

FROM debian:stretch-slim
WORKDIR /root/
COPY --from=builder /go/bin/conode .
COPY run_nodes.sh .
RUN mkdir /conode_data
RUN mkdir -p .local/share .config
RUN ln -s /conode_data .local/share/conode
RUN ln -s /conode_data .config/conode

EXPOSE 7770 7771

CMD "./run_nodes.sh -n 1"
