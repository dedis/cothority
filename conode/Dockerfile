FROM golang:1.8

WORKDIR /go/src/conode
COPY . .

RUN go-wrapper download   # "go get -d -v ./..."
RUN go-wrapper install    # "go install -v ./..."

EXPOSE 7002 7003

CMD ["./run_conode.sh", "public", "2"]
