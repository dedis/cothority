FROM golang:1.8

WORKDIR /go/src/github.com/dedis/logread
COPY . .

RUN go get -d -v ./conode
RUN go install -v ./conode
RUN echo $PATH
RUN which conode

EXPOSE 7003 7005 7007

# local - run this as a set of local nodes in the docker
# 3 - number of nodes to run
# 2 - debug-level: 0 - none .. 5 - a lot
# -wait - don't return from script when all nodes are started
CMD ["conode/run_conode.sh", "local",  "3", "2", "-wait" ]
