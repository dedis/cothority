FROM golang:1.9 as builder
RUN go get -d -v github.com/dedis/cothority/conode
RUN GOOS=linux go build -a -installsuffix cgo -o /conode \
  github.com/dedis/cothority/conode

FROM debian:stretch-slim
WORKDIR /root/
COPY --from=builder /conode .
RUN mkdir /conode_data
RUN mkdir -p .local/share .config
RUN ln -s /conode_data .local/share/conode
RUN ln -s /conode_data .config/conode

EXPOSE 6879 6880

CMD ["./conode", "-debug", "3", "server"]
