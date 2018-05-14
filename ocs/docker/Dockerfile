FROM golang:1.10 as builder
RUN go get -d -v github.com/dedis/cothority/
WORKDIR /go/src/github.com/dedis/cothority
RUN sed -i 's/_ "github.com\/dedis\/cothority\/evoting\/service"/_ "github.com\/dedis\/cothority\/ocs\/service"/g' conode/experimental.go
RUN go get -u ./...
RUN go install ./conode

EXPOSE 7003 7005 7007 7009

COPY co1/private.toml co1/private.toml
COPY co2/private.toml co2/private.toml
COPY co3/private.toml co3/private.toml
COPY co4/private.toml co4/private.toml

COPY co1/public.toml co1/public.toml
COPY co2/public.toml co2/public.toml
COPY co3/public.toml co3/public.toml
COPY co4/public.toml co4/public.toml

# local - run this as a set of local nodes in the docker
# 3 - number of nodes to run
# 2 - debug-level: 0 - none .. 5 - a lot
# -wait - don't return from script when all nodes are started
CMD ["env", "GODEBUG=gctrace=1", "conode/run_conode.sh", "local",  "4", "2", "-wait" ]
