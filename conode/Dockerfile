FROM golang:1.12 as builder
ARG BUILD_TAG=none
ARG ldflags="-s -w -X main.gitTag=unknown"
RUN go get go.dedis.ch/cothority
RUN cd /go/src/go.dedis.ch/cothority && git checkout $BUILD_TAG && GO111MODULE=on go install -ldflags="$ldflags" ./conode ./byzcoin/bcadmin ./eventlog/el ./scmgr 

FROM debian:stretch-slim
WORKDIR /root/
COPY ./docker-common.sh .
RUN ./docker-common.sh

COPY --from=builder /go/bin/conode .
COPY --from=builder /go/bin/bcadmin /go/bin/el /go/bin/scmgr /usr/local/bin/

EXPOSE 7770 7771

CMD ["/root/conode", "-d", "2", "server"]
