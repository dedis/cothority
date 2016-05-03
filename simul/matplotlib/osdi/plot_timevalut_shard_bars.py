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


# Plots a Cothority and a JVSS run with regard to their averages. Supposes that
# the last two values from JVSS are off-grid and writes them with arrows7
# directly on the plot
def plotResources():
    # prepare 2 Y-axis:
    fig, ax1 = plt.subplots()

    ax2 = ax1.twinx()
    ax1.plot(x, y1)
    ax2.plot(x, y2)

    ax1.set_xlabel('X data')
    ax1.set_ylabel('Y1 data')
    ax2.set_ylabel('Y2 data')

    mplot.plotPrepareLogLog()
    data = read_csvs('test_timevault')[0]
    plot_show('comparison_timevault')
    data.add_columns("round_open_bw_tx", "round_open_bw_rx")
    data.add_columns("round_seal_bw_tx", "round_seal_bw_rx")

    openBW_bar = mplot.plotBar(data, "round_open_bw_tx", "Bandwidth (Open)",
                        colors[0][0], delta_x=-0.5)
    sealBW_bar = mplot.plotBar(data, "round_seal_bw_tx", "Bandwidth (Seal)",
                        colors[0][1], delta_x=-0.25)
    sealCPU_bar = mplot.plotBar(data, "round_seal_user", "CPU (Seal)",
                                colors[1][0], delta_x=0.)
    openCPU_bar = mplot.plotBar(data, "round_open_user", "CPU (Open)",
                                colors[1][1], delta_x=0.25)



    # bar2 = mplot.plotBar(data, "bandwidth_rx", "Bandwidth (RX)",
    #                    colors[1][0], delta_x=0.5)


    plt.legend(loc=u'upper left')

    plt.ylabel("Resource Usage")
    plt.xlabel("Shard Size")
    # ax = plt.axes()
    # ax.set_xticks([2,8,32,128,512,2048,8192, 32768])
    mplot.plotEnd()


def arrow(x, y, label, dx=1., dy=1., text_align='left'):
    plt.annotate(label, xy=(x + dx / 10, y + dy / 10),
                 xytext=(x + dx / 2, y + dy / 2),
                 horizontalalignment=text_align,
                 arrowprops=dict(facecolor='black', headlength=5, width=0.1,
                                 headwidth=8))




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
#file_extension = 'eps'
# Show figure
mplot.show_fig = False

# Call all plot-functions
plotResources()
