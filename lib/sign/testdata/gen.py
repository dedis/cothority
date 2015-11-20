# Generate some relevant model graphs to use in simulation experiments

import math
import networkx as nx

n = 1000



# Generic random topologies

# Simple 3-regular random graphs: unrealistic but useful baseline
nx.write_weighted_edgelist(nx.random_regular_graph(3,n), "3reg.dat")

# Watts-Strogatz small-world graph
nx.write_weighted_edgelist(nx.watts_strogatz_graph(n,2,0.5), "ws.dat")

# Barabasi-Albert preferential attachment model
nx.write_weighted_edgelist(nx.barabasi_albert_graph(n,1), "ba.dat")

# Powerlaw cluster graph
nx.write_weighted_edgelist(nx.powerlaw_cluster_graph(n,1,0.1), "pc.dat")


# Planar geographic topologies

# Navigable small-world planar graph.
# Note that this produces nodes named by grid coords rather than integers.
#gridn = math.ceil(math.sqrt(n))
#nx.write_weighted_edgelist(nx.navigable_small_world_graph(gridn), "nsw.dat")

# Waxman planar graph
nx.write_weighted_edgelist(nx.waxman_graph(n), "wax.dat")

