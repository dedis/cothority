FROM golang:1.15-buster
RUN apt update && apt install -y procps ca-certificates && apt clean
WORKDIR /root/

EXPOSE 7770 7771 7772 7773 7774 7775 7776 7777 7778 7779 7780 7781 7782 7783

COPY co1/*.toml co1/
COPY co2/*.toml co2/
COPY co3/*.toml co3/
COPY co4/*.toml co4/
COPY co5/*.toml co5/
COPY co6/*.toml co6/
COPY co7/*.toml co7/
COPY conode /usr/local/bin/
COPY conode /root
COPY bcadmin /root
COPY csadmin /root
COPY run_nodes.sh /root

# local - run this as a set of local nodes in the docker
# 4 - number of nodes to run
# 2 - debug-level: 0 - none .. 5 - a lot
# -wait - don't return from script when all nodes are started
CMD ["env", "GODEBUG=gctrace=0", "COTHORITY_ALLOW_INSECURE_ADMIN=1", "./run_nodes.sh", "-n 4", "-v 2", "-t", "-f", "-s", "-c" ]
