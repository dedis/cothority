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
    ex_len = shortest_listlen(total, ex_list)
    ex_len_list = ex_list[0:ex_len]
    creation = 0
    p = 0.5 / math.pow(ex_len + 1, 0.65)
    #p = 0.5
    while True:
        creation += 1
        #pnew = p - float(creation) / total
        #print pnew
        bf = pybloomfilter.BloomFilter(total / 2, p, '/tmp/sda.bloom')
        for ex in ex_len_list:
            bf.add(ex)
        if false_positives(total, bf, ex_len_list):
            print total, ex_len, creation
        else:
            break

    return 32 + math.ceil(len(bf.to_base64()) * 6 / 8.0), creation, p

def false_positives(total, bf, ex_len_list):
        for ex in range(0, total):
            if (ex in bf) and not (ex in ex_len_list):
                return True
        return False

def calculate_exceptions(hosts):
    host_list = []
    for h in range(0, hosts):
        host_list.append(h)

    lengths = [[], [], [], [], [], []]
    for ex in range(0, hosts + 1, int(math.ceil(hosts / 64))):
        ex_list = random.sample(set(host_list), ex)
        lengths[0].append(ex)
        lengths[1].append(length_list(hosts, ex_list))
        lengths[2].append(length_bitmap(hosts, ex_list))
        length, creation, p = length_bloom(hosts, ex_list)
        #print ex, length, creation, p
        lengths[3].append(length)
        lengths[4].append(creation)
        lengths[5].append(p)
    return lengths

def plot_comparison(tree_size):
    mplot.plotPrepareLogLog(0, 0)
    lengths = calculate_exceptions(tree_size)
    plt.plot(lengths[0], lengths[1], linestyle='-', marker='v', color=color1_dark, label='Simple list')
    plt.plot(lengths[0], lengths[2], linestyle='-', marker='v', color=color2_dark, label='Bitmap')
    plt.plot(lengths[0], lengths[3], linestyle='-', marker='v', color=color3_dark, label='Bloom filter')
    plt.ylabel('Final message size')
    plt.xlabel('Elements sending exception')
    plt.title('Comparison with ' + str(tree_size) + ' elements')

    plt.legend(loc=u'upper right')
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    mplot.pngname = "comparison_exception_" +str(tree_size)+".png"
    mplot.plotEnd()

    mplot.plotPrepareLogLog(0,2)
    plt.plot(lengths[0], lengths[4], linestyle='-', marker='v', color=color1_dark, label='Recalculations')
    plt.plot(lengths[0], lengths[5], linestyle='-', marker='v', color=color2_dark, label='P')
    plt.ylabel('')
    plt.xlabel('Elements sending exception')
    plt.title('Comparison with ' + str(tree_size) + ' elements')

    plt.legend(loc=u'upper right')
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    mplot.pngname = "comparison_exception_recalc_" +str(tree_size)+".png"
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

#for size in [128, 2048, 16384]:
for size in [128, 2048, 4096]:
#for size in [128]:
    plot_comparison(size)
