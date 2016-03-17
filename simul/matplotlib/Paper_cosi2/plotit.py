#!/usr/bin/env python
# Plots the graph of one of the test-runs
# It takes the CSV-file as argument and shows the plot
# of the times used for each round

import os

os.environ["LC_ALL"] = "en_US.UTF-8"
os.environ["LANG"] = "en_US.UTF-8"

import sys

sys.path.insert(1, '..')
from mplot import MPlot
from stats import CSVStats
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches


def plotData(data, name,
             xlabel="Number of hosts", ylabel="Seconds per round",
             xticks=[], loglog=[2, 2], xname="hosts",
             legend_pos="lower right",
             yminu=0, ymaxu=0,
             title=""):
    mplot.plotPrepareLogLog(loglog[0], loglog[1])
    plots = read_csvs_xname(xname, *data[0])
    ranges = []
    data_label = []
    plot_show(name)

    for index, label in enumerate(data[1]):
        data_label.append([plots[index], label])
        ranges.append(
            mplot.plotMMA(plots[index], 'round_wall', colors[index][0], 4,
                          dict(label=label, linestyle='-', marker='o',
                               color=colors[index][1], zorder=5)))

    # Make horizontal lines and add arrows for JVSS
    xmin, xmax, ymin, ymax = CSVStats.get_min_max(*ranges)
    if yminu != 0:
        ymin = yminu
    if ymaxu != 0:
        ymax = ymaxu
    plt.ylim(ymin, ymax)
    # plt.xlim(16, xmax * 1.2)
    plt.xlabel(xlabel)
    plt.ylabel(ylabel)

    plt.legend(loc=legend_pos)
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    if len(xticks) > 0:
        ax = plt.axes()
        ax.set_xticks([16, 32, 64, 128, 256, 512, 1024, 4096, 16384, 65536])
    if title != "":
        plt.title(title)
    mplot.plotEnd()
    return data_label


# Plots a Cothority and a JVSS run with regard to their averages. Supposes that
# the last two values from JVSS are off-grid and writes them with arrows
# directly on the plot
def plotRoundtime():
    plotData([['jvss', 'naive_cosi', 'ntree_cosi', 'cosi_depth_3'],
              ['JVSS', 'Naive', 'NTree', 'CoSi']], 'comparison_roundtime',
             xticks=[4, 8, 16, 32, 64, 128, 256, 512, 1024, 4096, 16384, 65536],
             yminu=0.5, ymaxu=8)


# Plots a Cothority and a JVSS run with regard to their averages. Supposes that
# the last two values from JVSS are off-grid and writes them with arrows
# directly on the plot
def plotSysUser():
    mplot.plotPrepareLogLog()
    plots = read_csvs('jvss', 'naive_cosi', 'sysusr_ntree', 'sysusr_cosi')
    plot_show('comparison_sysusr')

    ymin = 0.05
    bars = []
    deltax = -2
    for index, label in enumerate(['JVSS', 'Naive', 'NTree', 'CoSi']):
        bars.append(mplot.plotStackedBarsHatched(plots[index], "round_system",
                                                 "round_user", label,
                                                 colors[index][0],
                                                 ymin, delta_x=deltax + index)[
                        0])

    ymax = 64
    xmax = 3192
    plt.ylim(ymin, ymax)
    plt.xlim(1.5, xmax)

    usert = mpatches.Patch(color='white', ec='black', label='User time',
                           hatch='//')
    syst = mpatches.Patch(color='white', ec='black', label='System time')

    plt.legend(handles=[bars[0], bars[1], bars[2], bars[3], usert, syst],
               loc=u'upper left')
    plt.ylabel("Average seconds per round")
    mplot.plotEnd()


# Plots the branching factor
def plotBF():
    data_label = plotData([['cosi_bf_2048', 'cosi_bf_4096', 'cosi_bf_8192'],
                           ['2048 Hosts', '4096 Hosts', '8192 Hosts']],
                          'cosi_bf',
                          xname='bf',
                          xlabel="Branching factor",
                          loglog=[0, 0],
                          legend_pos="upper right")

    if False:
        for index, data_label in enumerate(data_label):
            data, label = data_label
            plt.plot(data.x, data.columns['depth'], linestyle=':', marker='v',
                     color=colors[index][1],
                     label='CoSi ' + label + ' depth')



    plt.legend(loc='upper right')
    mplot.plotEnd()


# Plots the oversubscription
def plotOver():
    plotData([['cosi_over_1', 'cosi_over_2', 'cosi_over_3'],
              ['8 servers', '16 servers', '32 servers']], 'cosi_over',
             ylabel="Seconds per round",
             legend_pos = "upper left")


def plotNetwork():
    mplot.plotPrepareLogLog(2, 10)
    plots = read_csvs('jvss', 'naive_cosi', 'ntree_cosi_check_none',
                      'cosi_depth_3')
    plot_show('comparison_network')

    for index, label in enumerate(['JVSS', 'Naive', 'NTree', 'CoSi']):
        bandwidth = []
        data = plots[index]
        bw_tx = data.columns['bandwidth_root_tx_sum']
        bw_rx = data.columns['bandwidth_root_rx_sum']
        for p in range(0, len(bw_tx)):
            bandwidth.append((bw_tx[p] + bw_rx[p]) / 1000)
        plt.plot(data.x, bandwidth, label=label, linestyle='-', marker='o',
                 color=colors[index][1])

    # Make horizontal lines and add arrows for JVSS
    plt.ylabel('Total network-traffic [kBytes]')

    plt.legend(loc=u'lower right')
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    mplot.plotEnd()
    return


def plotCheckingNtree():
    plotData(
        [['ntree_cosi', 'ntree_cosi_check_simple', 'ntree_cosi_check_none'],
         ['NTree check all', 'NTree check children', 'NTree check none']],
        'comparison_ntree',
        ylabel="Seconds per round",
        legend_pos="upper left")


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
file_extension = 'eps'
# Show figure
mplot.show_fig = False

# Call all plot-functions
plotRoundtime()
plotSysUser()
plotBF()
plotOver()
plotNetwork()
plotCheckingNtree()
