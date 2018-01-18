FROM golang:1.8

WORKDIR /go/src/github.com/dedis/onchain-secrets
COPY . .

RUN go get -d -v ./conode

RUN git --git-dir=/go/src/github.com/dedis/onet/.git log -1  --format=oneline
RUN git --git-dir=/go/src/github.com/dedis/cothority/.git log -1 --format=oneline
RUN git --git-dir=/go/src/github.com/dedis/onchain-secrets/.git log -1 --format=oneline
RUN git --git-dir=/go/src/github.com/dedis/kyber/.git log -1 --format=oneline

# TODO remove when bftcosi_failure is in master
RUN git --git-dir=/go/src/github.com/dedis/cothority/.git --work-tree=/go/src/github.com/dedis/cothority checkout bftcosi_failure

RUN go install -v ./conode
RUN echo $PATH
RUN which conode

# local - run this as a set of local nodes in the docker
# 3 - number of nodes to run
# 2 - debug-level: 0 - none .. 5 - a lot
# -wait - don't return from script when all nodes are started
