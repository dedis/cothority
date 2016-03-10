#!/usr/bin/env python
# Plots the graph of one of the test-runs
# It takes the CSV-file as argument and shows the plot
# of the times used for each round

import os
os.environ["LC_ALL"] = "en_US.UTF-8"
os.environ["LANG"] = "en_US.UTF-8"

import sys
sys.path.insert(1,'..')
from mplot import MPlot
from stats import CSVStats
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches


# Plots a Cothority and a JVSS run with regard to their averages. Supposes that
# the last two values from JVSS are off-grid and writes them with arrows
# directly on the plot
def plotCoSiOld():
    mplot.plotPrepareLogLog()
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
    #plt.plot(cosi_old.x, co_old_depth, linestyle='-', marker='v', color=color5_dark, label='CoSi old depth')

    # Make horizontal lines and add arrows for JVSS
    # xmin, xmax, ymin, ymax = CSVStats.get_min_max(na, co)
    xmin, xmax, ymin, ymax = CSVStats.get_min_max(na_co, nt_co, jv, co_3, co_3_c)
    plt.ylim(0.5, 8)
    plt.xlim(16, xmax * 1.2)
    plt.ylabel('Seconds per round')

    plt.legend(loc=u'lower right')
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
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
file_extension = 'png'

def check_Args(args):
    if len(sys.argv) < args + 1:
        print("Error: Please give a mode and " + str(args) + " .csv-files as argument - " + str(len(sys.argv)) + "\n")
        print("Mode: (0=printAverage, 1=printSystemUserTimes with bars, 2=printSystemUserTimes with areas)\n")
        print("CSV: cothority.csv jvss.csv\n")
        exit(1)

def plot_show(file):
    if write_file:
        mplot.pngname = file + '.' + file_extension
        mplot.show_fig = False

def read_csvs_xname(xname, *values):
    stats = []
    for a in values:
        print "Reading " + a
        stats.append(CSVStats(a + '.csv', xname))
    return stats

def read_csvs(*values):
    return read_csvs_xname("hosts", *values)

# Call all plot-functions
plotCoSiOld()
