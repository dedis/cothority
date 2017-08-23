FROM golang:1.8

WORKDIR /go/src/conode
COPY . .

RUN go-wrapper download   # "go get -d -v ./..."
#RUN go-wrapper install    # "go install -v ./..."
RUN cd /go/src/gopkg.in/dedis/onet.v1; git checkout portscan_212

EXPOSE 6879 6880

CMD ["./run_conode.sh", "public", "2"]
