#!/usr/bin/env python
# Simple plot-binary for showing one graph. Syntax is:
#
#   plot.py file.csv [stat]
#
# where file.csv is the file to print and stat is an optional argument taking
# one of {round_wall, round_system, round_user} or another statistic present
# in the csv-file.

import sys
from mplot import MPlot
from stats import CSVStats
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches

# Colors for the Cothority
color1_light = 'lightgreen'
color1_dark = 'green'
color2_light = 'lightblue'
color2_dark = 'blue'
color3_light = 'yellow'
color3_dark = 'brown'
color4_light = 'pink'
color4_dark = 'red'
mplot = MPlot()

def plot_save(file):
    mplot.pngname = file
    mplot.show_fig = False

stats = "round_wall"
argn = len(sys.argv)
if argn < 2:
    print "Syntax is:"
    print "plot.py file.csv [stat]"
    print "where stat is one of {round_wall, round_system, round_user}"
    sys.exit(1)

if argn > 2:
    stats = sys.argv[2]

data = CSVStats(sys.argv[1], "hosts")
mplot.plotPrepareLogLog()
na = mplot.plotMMA(data, stats, color1_light, 0,
                    dict(label=sys.argv[1], linestyle='-', marker='s', color=color2_dark, zorder=3))
plt.legend(loc='upper left')
plt.ylabel('Time for verification')
mplot.plotEnd()