FROM golang:1.10 as builder
RUN apt-get update && apt-get install -y clang
RUN go get golang.org/x/vgo
RUN go get github.com/dedis/cothority
COPY .netrc /root
RUN cd /go/src/github.com/dedis/cothority/conode && GOOS=linux vgo install

FROM debian:stretch-slim
WORKDIR /root/
COPY --from=builder /go/bin/conode .
COPY setup-then-start.sh .
RUN mkdir /conode_data
RUN mkdir -p .local/share .config
RUN ln -s /conode_data .local/share/conode
RUN ln -s /conode_data .config/conode

EXPOSE 6879 6880

CMD "./setup-then-start.sh"
