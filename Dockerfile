FROM golang:1.8

WORKDIR /go/src/github.com/dedis/onchain-secrets
COPY . .

RUN go get -d -v ./conode

RUN ./show_deps.sh

RUN go install -v ./conode
RUN echo $PATH
RUN which conode

