FROM dedis/conode:dev

# EXPOSE 7771 7773 7775 7777 7779 7781 7783

COPY co1/*.toml co1/
COPY co2/*.toml co2/
COPY co3/*.toml co3/
COPY co4/*.toml co4/
COPY co5/*.toml co5/
COPY co6/*.toml co6/
COPY co7/*.toml co7/

# local - run this as a set of local nodes in the docker
# 4 - number of nodes to run
# 2 - debug-level: 0 - none .. 5 - a lot
# -wait - don't return from script when all nodes are started
RUN cp ./conode /usr/local/bin/
CMD ["env", "GODEBUG=gctrace=0", "COTHORITY_ALLOW_INSECURE_ADMIN=1", "./run_nodes.sh", "-n 4", "-v 2", "-t", "-f", "-s" ]
