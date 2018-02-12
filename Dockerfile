FROM golang:1.8

WORKDIR /go/src/github.com/dedis/onchain-secrets
COPY . .

RUN curl -fsSL -o /usr/local/bin/dep \
        https://github.com/golang/dep/releases/download/v0.4.1/dep-linux-amd64 && \
    echo "31144e465e52ffbc0035248a10ddea61a09bf28b00784fd3fdd9882c8cbb2315  /usr/local/bin/dep" | sha256sum -c && \
    chmod +x /usr/local/bin/dep && \
    dep ensure -vendor-only -v

RUN go install ./conode
RUN echo $PATH
RUN which conode

