FROM golang:1.9

WORKDIR /go/src/github.com/dedis/onchain-secrets
COPY . .

RUN go get -t -v ./conode && go install -v ./conode
RUN echo $PATH
RUN which conode

