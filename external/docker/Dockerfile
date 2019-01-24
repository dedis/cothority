FROM dedis/conode:dev

# EXPOSE 7003 7005 7007 7009

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
CMD ["env", "GODEBUG=gctrace=0", "DEBUG_TIME=true", "COTHORITY_ALLOW_INSECURE_ADMIN=1", "./run_nodes.sh", "4", "2" ]
