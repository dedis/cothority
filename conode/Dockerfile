FROM golang:1.8

WORKDIR /go/src/conode
COPY . .

RUN go-wrapper download   # "go get -d -v ./..."
RUN go-wrapper install    # "go install -v ./..."

EXPOSE 6879 6880

CMD ["./run_conode.sh", "public", "2"]
