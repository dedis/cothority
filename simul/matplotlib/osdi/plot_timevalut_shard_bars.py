#!/usr/bin/env python

import os

os.environ["LC_ALL"] = "en_US.UTF-8"
os.environ["LANG"] = "en_US.UTF-8"

import sys

sys.path.insert(1, '..')
from mplot import MPlot
from stats import CSVStats
import matplotlib.pyplot as plt


# Plots resource usage compared to shard size
def plotResources():

    data = read_csvs('test_timevault')[0]
    plot_show('comparison_timevault')

    # add both bandwidth measurements (TX+RX) to get the total BW:
    data.add_columns("round_open_bw_tx", "round_open_bw_rx")
    data.add_columns("round_seal_bw_tx", "round_seal_bw_rx")
    # divide by 1000 ->
    data.column_mul("round_open_bw_tx", 0.001)
    data.column_mul("round_seal_bw_tx", 0.001)

    fig, ax1 = plt.subplots()
    ax2 = ax1.twinx()
    # logarithmic y-axes
    ax1.set_yscale("log", basey=10)
    ax2.set_yscale("log", basey=10)

    ax1.set_ylabel("Bandwidth Usage (KB)")
    ax2.set_ylabel('Resource Usage (Seconds)')
    width = 0.25

    val = data.get_values("round_seal_bw_tx")
    y = val.avg
    # init positions of the bars:
    x = val.x
    pos = list(range(len(x)))
    ax1.bar(pos,
            y,
            width,
            color='green',
            label="Bandwidth (Seal)")


    val = data.get_values("round_open_bw_tx")
    y = val.avg
    ax1.bar([p + 0.5*width for p in pos],
           y,
           width,
           color='lightgreen',
           label="Bandwidth (Open)")


    val = data.get_values("round_seal_user")
    y = val.avg
    ax2.bar([p + width for p in pos],
            y,
            width,
            color='lightblue',
            label="CPU (Seal)")

    val = data.get_values("round_open_user")
    y = val.avg
    ax2.bar([p + 1.5*width for p in pos],
            y,
            width,
            color='blue',
            label="CPU (Open)")


    ax1.legend(loc='upper left')
    # transform the location of the legend:
    ax2.legend(loc='center left', bbox_to_anchor=(0., 0.75))

    ax1.set_xticks([p + 1. * width for p in pos])
    # Set the labels for the x ticks (4, 8, 16, ...)
    ax1.set_xticklabels([int(i) for i in x])
    # common label of axes
    ax1.set_xlabel("Shard Size")

    mplot.plotEnd()

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
