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
def plotCoSi():
    mplot.plotPrepareLogLog()
    plots = read_csvs('jvss', 'naive_cosi', 'ntree_cosi', 'cosi_depth_3')
    ranges = []
    plot_show('comparison_roundtime')

    for index, label in enumerate(['JVSS', 'Naive', 'NTree', 'CoSi']):
        ranges.append(mplot.plotMMA(plots[index], 'round_wall', colors[index][0], 4,
                           dict(label=label, linestyle='-', marker='o', color=colors[index][1], zorder=5)))

    # Make horizontal lines and add arrows for JVSS
    xmin, xmax, ymin, ymax = CSVStats.get_min_max(*ranges)
    plt.ylim(0.5, 8)
    plt.xlim(16, xmax * 1.2)
    plt.ylabel('Seconds per round')

    plt.legend(loc=u'lower right')
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    ax = plt.axes()
    ax.set_xticks([16, 32, 64, 128, 256, 512, 1024, 4096, 16384, 65536])
    mplot.plotEnd()


# Plots a Cothority and a JVSS run with regard to their averages. Supposes that
# the last two values from JVSS are off-grid and writes them with arrows
# directly on the plot
def plotCoSiSysUser():
    mplot.plotPrepareLogLog()
    plots = read_csvs('jvss', 'naive_cosi', 'sysusr_ntree', 'sysusr_cosi')
    plot_show('comparison_sysusr')

    ymin = 0.05
    bars = []
    deltax = -2
    for index, label in enumerate(['JVSS', 'Naive', 'NTree', 'CoSi']):
        bars.append( mplot.plotStackedBarsHatched(plots[index], "round_system", "round_user", label, colors[index][0],
                                                      ymin, delta_x=deltax +index)[0])

    ymax = 7
    xmax = 3192
    plt.ylim(ymin, ymax)
    plt.xlim(1.5, xmax)

    usert = mpatches.Patch(color='white', ec='black', label='User time', hatch='//')
    syst = mpatches.Patch(color='white', ec='black', label='System time')

    plt.legend(handles=[bars[0], bars[1], bars[2], bars[3], usert, syst], loc=u'upper left')
    mplot.plotEnd()


# Plots the branching factor
def plotBF():
    mplot.plotPrepareLogLog(0, 0)
    cosi_bf = read_csvs_xname('bf', 'cosi_bf_2048',
                                                               'cosi_bf_4096', 'cosi_bf_8192')
    plot_show('cosi_bf')
    cbf = []

    for index, label in enumerate(['2048', '4096', '8192']):
        data = cosi_bf[index]
        cbf.append(mplot.plotMMA(data, 'round_wall', colors[index][0], 4,
                       dict(label='CoSi ' + label, linestyle='-', marker='o', color=colors[index][0], zorder=5)))
        plt.plot(data.x, data.columns['depth'], linestyle=':', marker='v', color=colors[index][1],
                     label='CoSi ' + label + ' depth')

    xmin, xmax, ymin, ymax = CSVStats.get_min_max(cbf[0], cbf[1], cbf[2])
    plt.ylim(ymin, ymax)
    plt.xlim(xmin, xmax * 1.2)
    plt.ylabel('Seconds per round')
    plt.xlabel('Branching factor')

    plt.legend(loc=u'lower right')
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    mplot.plotEnd()


# Plots the oversubscription
def plotOver():
    mplot.plotPrepareLogLog()
    plots = read_csvs('cosi_over_1', 'cosi_over_2', 'cosi_over_3')
    plot_show('cosi_over')

    ranges = []
    for index, label in enumerate(['8', '16', '4']):
        ranges.append(mplot.plotMMA(plots[index], 'round_wall', colors[index][0], 4,
                       dict(label='Cosi ' + label + ' servers', linestyle='-', marker='o',
                            color=colors[index][1], zorder=5)))

    xmin, xmax, ymin, ymax = CSVStats.get_min_max(*ranges)
    plt.ylim(ymin, ymax)
    plt.xlim(xmin, xmax * 1.2)
    plt.ylabel('Seconds per round')

    plt.legend(loc=u'lower right')
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    mplot.plotEnd()

def plotNetwork():
    return

def plotChecking():
    return

# Colors for the Cothority
colors = [['lightgreen', 'green'],
          ['lightblue', 'blue'],
          ['yellow', 'brown'],
          ['pink', 'red'],
          ['pink', 'red']]
mplot = MPlot()

def plot_show(file):
    if write_file:
        mplot.pngname = file + '.' + file_extension

def read_csvs_xname(xname, *values):
    stats = []
    for a in values:
        file = a + '.csv'
        stats.append(CSVStats(file, xname))
    return stats

def read_csvs(*values):
    return read_csvs_xname("hosts", *values)

# Write to file
write_file = True
# What file extension - .png, .eps
file_extension = 'png'
# Show figure
mplot.show_fig = False

# Call all plot-functions
plotCoSi()
plotCoSiSysUser()
plotBF()
plotOver()
#plotNetwork()
#plotChecking()
