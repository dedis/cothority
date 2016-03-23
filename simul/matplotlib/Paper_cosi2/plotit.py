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
             xlabel="Number of witnesses", ylabel="Signing round latency in seconds",
             xticks=[], loglog=[2, 2], xname="hosts",
             legend_pos="lower right",
             yminu=0, ymaxu=0,
             xminu=0, xmaxu=0,
             title="", read_plots=True):
    mplot.plotPrepareLogLog(loglog[0], loglog[1])
    if read_plots:
        plots = read_csvs_xname(xname, *data[0])
    else:
        plots = data[0]

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
    if xminu != 0:
        xmin = xminu
    if xmaxu != 0:
        xmax = xmaxu
    plt.ylim(ymin, ymax)
    plt.xlim(xmin, xmax)
    plt.xlabel(xlabel)
    plt.ylabel(ylabel)

    plt.legend(loc=legend_pos)
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    if len(xticks) > 0:
        ax = plt.axes()
        ax.set_xticks(xticks)
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
             xticks=[2, 8, 32, 128, 512, 2048, 8192, 32768],
             yminu=0.5, ymaxu=8, xminu=4)
    arrow(8000, 2.1, "oversubscription", -6000, 0.9, 'right')
    mplot.plotEnd()


# Plots a Cothority and a JVSS run with regard to their averages. Supposes that
# the last two values from JVSS are off-grid and writes them with arrows7
# directly on the plot
def plotSysUser():
    mplot.plotPrepareLogLog()
    plots = read_csvs('jvss', 'naive_cosi', 'sysusr_ntree', 'sysusr_cosi')
    plot_show('comparison_sysusr')

    if False:
        for index in range(1, len(plots[3].x)):
            for p in range(0, len(plots)):
                if index < len(plots[p].x):
                    plots[p].delete_index(index)

    ymin = 0.05
    bars = []
    deltax = -1.5
    for index, label in enumerate(['JVSS', 'Naive', 'NTree', 'CoSi']):
        bars.append(mplot.plotStackedBarsHatched(plots[index], "round_system",
                                                 "round_user", label,
                                                 colors[index][0],
                                                 ymin, delta_x=deltax + index)[
                        0])

    ymax = 32
    xmax = 50000
    plt.ylim(ymin, ymax)
    plt.xlim(1, xmax)

    usert = mpatches.Patch(color='white', ec='black', label='User',
                           hatch='//')
    syst = mpatches.Patch(color='white', ec='black', label='System')

    plt.legend(handles=[bars[0], bars[1], bars[2], bars[3], usert, syst],
               loc=u'upper left')
    plt.ylabel("Average CPU seconds per round")
    ax = plt.axes()
    ax.set_xticks([2,8,32,128,512,2048,8192, 32768])
    mplot.plotEnd()


# Plots the branching factor
def plotBF():
    data_label = plotData([['cosi_bf_2048', 'cosi_bf_4096', 'cosi_bf_8192'],
                           ['2048 Witnesses', '4096 Witnesses', '8192 Witnesses']],
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

    arrow(6, 3, "depth=5")
    arrow(10, 2.5, "depth=4")
    arrow(6, 2.53, "depth=5", 0.7, 0.5, "center")
    arrow(8, 2.2, "depth=4", 0.6, 0.6, "center")
    arrow(16, 1.7, "depth=3", text_align="center")
    arrow(5.2, 2.4, "depth=5", -3., -1.)
    arrow(7.2, 1.95, "depth=4", -3., -0.8)
    arrow(13, 1.55, "depth=3", 0.6, 0.6, "center")

    plt.legend(loc='upper right')
    plt.xlim(2, 18)
    mplot.plotEnd()


def arrow(x, y, label, dx=1., dy=1., text_align='left'):
    plt.annotate(label, xy=(x + dx / 10, y + dy / 10),
                 xytext=(x + dx / 2, y + dy / 2),
                 horizontalalignment=text_align,
                 arrowprops=dict(facecolor='black', headlength=5, width=0.1,
                                 headwidth=8))


# Plots the oversubscription
def plotOver():
    plots = read_csvs('cosi_over_1', 'cosi_over_2', 'cosi_depth_3')

    for p in [0, 1]:
        plots[p].column_add('round_wall', 0.05)

    plotData([plots,
              ['Witnesses split over 8 physical machines',
               'Witnesses split over 16 physical machines',
               'Witnesses split over 32 physical machines']],
             'cosi_over',
             xlabel="Total number of witnesses",
             legend_pos="upper left",
             read_plots=False)


# Plots the oversubscription with shifted graphs
def plotOver2():
    plots = read_csvs('cosi_over_1', 'cosi_over_2', 'cosi_over_3')

    for index in range( 0, len(plots[0].x )):
        plots[0].x[index] /= 8
        plots[1].x[index] /= 16
        plots[2].x[index] /= 32
    plotData([plots,
              ['8 servers', '16 servers', '32 servers']], 'cosi_over_2',
             xlabel="Number of witnesses per server",
             legend_pos="upper left",
             read_plots=False)


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
    plt.ylabel('Total network traffic (kBytes)')

    plt.legend(loc=u'lower right')
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    mplot.plotEnd()
    return


def plotCheckingNtree():
    plotData(
        [['ntree_cosi', 'ntree_cosi_check_simple', 'ntree_cosi_check_none'],
         ['NTree check all', 'NTree check children', 'NTree check none']],
        'comparison_ntree',
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
file_extension = 'png'
file_extension = 'eps'
# Show figure
mplot.show_fig = False

# Call all plot-functions
plotRoundtime()
plotSysUser()
plotBF()
plotOver()
plotOver2()
plotNetwork()
plotCheckingNtree()
