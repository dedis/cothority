FROM golang:1.12 as builder
ARG BUILD_TAG=none
ARG ldflags="-s -w -X main.gitTag=unknown"
RUN go get go.dedis.ch/cothority
RUN cd /go/src/go.dedis.ch/cothority && git checkout $BUILD_TAG && GO111MODULE=on go install -ldflags="$ldflags" ./conode ./byzcoin/bcadmin ./eventlog/el ./scmgr 

FROM debian:stretch-slim
RUN apt update; apt install -y procps ca-certificates; apt clean
WORKDIR /root/
RUN mkdir /conode_data
RUN mkdir -p .local/share .config
RUN ln -s /conode_data .local/share/conode
RUN ln -s /conode_data .config/conode
COPY --from=builder /go/bin/conode .
COPY --from=builder /go/bin/bcadmin /go/bin/el /go/bin/scmgr /usr/local/bin/

EXPOSE 7770 7771

CMD ["/root/conode", "-d", "2", "server"]
