#!/usr/bin/env python
# Calculates the size of an exception-message, based on 32 bytes for
# Challenge and Response, plus
# 1 - a list of indexes as varints to a list of public keys
# 2 - a bitmap
# 3 - a bloom-filter with probability of 0.01
#

import os
os.environ["LC_ALL"] = "en_US.UTF-8"
os.environ["LANG"] = "en_US.UTF-8"

import sys
sys.path.insert(1,'..')
from mplot import MPlot
from stats import CSVStats
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
import math
import random
try:
    import pybloomfilter
except ImportError:
    print "Please install Pybloomfilter:"
    print "\nsudo pip install pybloomfiltermmap\n"
    sys.exit(1)

# Plots a Cothority and a JVSS run with regard to their averages. Supposes that
# the last two values from JVSS are off-grid and writes them with arrows
# directly on the plot
def plotCoSi():
    cosi_old, cosi_3, cosi_3_check, jvss, naive_cosi, ntree_cosi = \
        read_csvs('cosi_old', 'cosi_depth_3', 'cosi_depth_3_check', 'jvss', 'naive_cosi', 'ntree_cosi')
    plot_show('comparison_roundtime')

    jv = mplot.plotMMA(jvss, 'round_wall', color2_light, 4,
                       dict(label='JVSS', linestyle='-', marker='o', color=color2_dark, zorder=5))

    na_co = mplot.plotMMA(naive_cosi, 'round_wall', color3_light, 4,
                       dict(label='Naive', linestyle='-', marker='o', color=color3_dark, zorder=5))

    nt_co = mplot.plotMMA(ntree_cosi, 'round_wall', color4_light, 4,
                       dict(label='NTree', linestyle='-', marker='o', color=color4_dark, zorder=5))

    co_3_c = mplot.plotMMA(cosi_3_check, 'round_wall', color1_light, 4,
                       dict(label='CoSi check - depth 3', linestyle='-', marker='v', color=color1_dark, zorder=5))

    co_3 = mplot.plotMMA(cosi_3, 'round_wall', color1_light, 4,
                       dict(label='CoSi - depth 3', linestyle='-', marker='o', color=color1_dark, zorder=5))

    #co_old = mplot.plotMMA(cosi_old, 'round_wall', color5_light, 4,
    #                   dict(label='CoSi old', linestyle='-', marker='s', color=color5_dark, zorder=5))
    #co_old_depth = cosi_old.get_old_depth()

    # Make horizontal lines and add arrows for JVSS
    # xmin, xmax, ymin, ymax = CSVStats.get_min_max(na, co)
    xmin, xmax, ymin, ymax = CSVStats.get_min_max(na_co, nt_co, jv, co_3, co_3_c)
    plt.ylim(0.5, 8)
    plt.xlim(16, xmax * 1.2)

def varint_len(i):
    if i == 0:
        return 1.0
    bits = math.ceil(math.log(math.fabs(i)+ 1, 2))
    return math.ceil(bits / 7)

def shortest_listlen(total, ex_list):
    ex_len = len(ex_list)
    if ex_len > total / 2:
        ex_len = total - ex_len
    return ex_len

def length_list(total, ex_list):
    # Challenge + Response
    length = 32
    ex_len = shortest_listlen(total, ex_list)
    for ex in range(0, ex_len):
        length += varint_len(ex_list[ex])
    return length

def length_bitmap(total, ex_list):
    return 32 + math.ceil(total / 8.0)

def length_bloom(total, ex_list):
    bf = pybloomfilter.BloomFilter(total, 0.5, '/tmp/sda.bloom')
    ex_len = shortest_listlen(total, ex_list)
    for ex in range(0, ex_len):
        bf.add(ex)
    return 32 + math.ceil(len(bf.to_base64()) * 6 / 8.0)

def calculate_exceptions(hosts):
    host_list = []
    for h in range(0, hosts):
        host_list.append(h)

    lengths = [[], [], [], []]
    for ex in range(0, hosts + 1, int(math.ceil(hosts / 64))):
        ex_list = random.sample(set(host_list), ex)
        lengths[0].append(ex)
        lengths[1].append(length_list(hosts, ex_list))
        lengths[2].append(length_bitmap(hosts, ex_list))
        lengths[3].append(length_bloom(hosts, ex_list))
    return lengths

def plot_comparison(tree_size):
    mplot.plotPrepareLogLog(0, 0)
    lengths = calculate_exceptions(tree_size)
    plt.plot(lengths[0], lengths[1], linestyle='-', marker='v', color=color1_dark, label='Simple list')
    plt.plot(lengths[0], lengths[2], linestyle='-', marker='v', color=color2_dark, label='Bitmap')
    plt.plot(lengths[0], lengths[3], linestyle='-', marker='v', color=color3_dark, label='Bloom filter')
    plt.ylabel('Final message size')
    plt.title('Comparison with ' + str(tree_size) + ' elements')

    plt.legend(loc=u'upper right')
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    ax = plt.axes()
    mplot.pngname = "comparison_exception_" +str(tree_size)+".png"
    mplot.plotEnd()


# Colors for the Cothority
color1_light = 'lightgreen'
color1_dark = 'green'
color2_light = 'lightblue'
color2_dark = 'blue'
color3_light = 'yellow'
color3_dark = 'brown'
color4_light = 'pink'
color4_dark = 'red'
color5_light = 'pink'
color5_dark = 'red'
mplot = MPlot()
write_file = True
mplot.show_fig = False

for size in [128, 16384, 131072]:
    plot_comparison(size)
