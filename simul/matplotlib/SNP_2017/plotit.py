#!/usr/bin/env python
# Plots the graph of one of the test-runs
# It takes the CSV-file as argument and shows the plot
# of the times used for each round

import os

os.environ["LC_ALL"] = "en_US.UTF-8"
os.environ["LANG"] = "en_US.UTF-8"

import sys

sys.path.insert(1, '.')
from mplot import MPlot
from stats import CSVStats
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
import numpy as np

import csv


def plotData(data, name,
             xlabel="Number of witnesses", ylabel="Signing round latency in seconds",
             xticks=[], loglog=[2, 2], xname="hosts",
             legend_pos="lower right",
             yminu=0, ymaxu=0,
             xminu=0, xmaxu=0,
             title="", read_plots=True,
             csv_column="round_wall"):
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
            mplot.plotMMA(plots[index], csv_column, colors[index], 4,
                          dict(label=label, linestyle='-', marker='o',
                               color=colors[index], alpha=alpha, zorder=5)))

    # Make horizontal lines and add arrows for tss
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


def arrow(x, y, label, dx=1., dy=1., text_align='left'):
    plt.annotate(label, xy=(x + dx / 10, y + dy / 10),
                 xytext=(x + dx / 2, y + dy / 2),
                 horizontalalignment=text_align,
                 arrowprops=dict(facecolor='black', headlength=5, width=0.1,
                                 headwidth=8))


def plotRandHerdSetup(timeStr):
    plot_show("randherd_setup_" + timeStr)
    mplot.plotPrepareLogLog(0, 10)
    width = 0.2
    values = len(hosts)
    x = np.arange(values)
    handles = []
    labels = ['RandHound', 'TSS-CoSi', 'CoSi']
    for index, groupSize in enumerate(groupSizes):
        cosi = getWallCPUAvg(plots[2], 'round', timeStr)
        rhound = getWallCPUAvg(plots[0], 'tgen-randhound', timeStr, 'groupsize', groupSize)
        tsscosi = getWallCPUAvg(plots[1], 'setup', timeStr, 'groupsize', groupSize)

        h1 = plt.bar(x+(index-1.5)*width, cosi, width, color=colorsbar[1], alpha=alphabar)
        h2 = plt.bar(x+(index-1.5)*width, tsscosi, width, color=colorsbar[2], alpha=alphabar,
                bottom=cosi)
        h3 = plt.bar(x+(index-1.5)*width, rhound, width, color=colorsbar[0], alpha=alphabar,
                bottom=cosi+tsscosi)

        if index == 0:
            handles = [h3, h2, h1]
            ymin = cosi[0]

        if index == len(groupSizes) - 1:
            lastx = values - 1
            ymax = cosi[lastx] + tsscosi[lastx] + rhound[lastx]

    plt.legend(handles , labels, loc='upper left',
               prop={'size':10})
    plt.ylabel(timeStr + "-Time for Setting up RandHerd (sec)")
    plt.ylim(ymin / 5, ymax * 5)
    plotNodesGroupSizes(x, width)
    mplot.plotEnd()

def plotRandHerdRound():
    plot = snp17tsscosi
    plot_show("randherd_round")
    mplot.plotPrepareLogLog(0, 0)
    gs = groupSizes.tolist()
    gs.reverse()
    for index, groupSize in enumerate(gs):
        y = plot.get_values_filtered("round_wall", "groupsize", groupSize)
        plt.plot(y.x[0], y.avg + 2, color=colorsplot[index], alpha=alpha, marker="o",
                 label="%d" % groupSize)
    plt.axes().set_xticks(hosts)
    plt.legend(loc="upper left", title="Group Size", ncol=2)
    plt.ylabel("Wall-clock Time for one RandHerd Round (sec)")
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    mplot.plotEnd()

def plotRandHound(timeStr):
    plot = snp17rhound
    plot_show("randhound_" + timeStr)
    mplot.plotPrepareLogLog(0, 0)
    width = 0.2
    values = len(hosts)
    x = np.arange(values)
    handles = []
    labels = ['Verification', 'Generation', 'Server']
    for index, groupSize in enumerate(groupSizes):
        server = getWallCPUAvg(plot, 'randhound_server_i1', timeStr, 'groupsize', groupSize)
        server += getWallCPUAvg(plot, 'randhound_server_i2', timeStr, 'groupsize', groupSize)
        server *= hosts
        generation = getWallCPUAvg(plot, 'tgen-randhound', timeStr, 'groupsize', groupSize)
        verification = getWallCPUAvg(plot, 'tver-randhound', timeStr, 'groupsize', groupSize)

        h1 = plt.bar(x+(index-1.5)*width, server, width, color=colorsbar[0], alpha=alphabar)
        h2 = plt.bar(x+(index-1.5)*width, generation, width, color=colorsbar[1], alpha=alphabar,
                     bottom=server)
        h3 = plt.bar(x+(index-1.5)*width, verification, width, color=colorsbar[2], alpha=alphabar,
                     bottom=server + generation)

        if index == 0:
            handles = [h3, h2, h1]
            ymin = server[0]

        if index == len(groupSizes) - 1:
            lastx = values - 1
            ymax = server[lastx] + generation[lastx] + verification[lastx]

    plt.legend(handles, labels, loc='upper left',
               prop={'size':10})
    plt.ylabel(timeStr + "-Time for Different Group Sizes (sec)")
    plt.ylim(ymin, ymax * 1.1)
    plotNodesGroupSizes(x, width)
    mplot.plotEnd()

def plotNodesGroupSizes(x, width):
    values = len(hosts)
    ax1 = plt.axes()
    ax1.set_xlim(-0.5, values - 0.3)
    xticks = []
    for i in range(len(groupSizes)):
        xticks += (x + (i-1) * width).tolist()
    xticks.sort()
    ax1.set_xticks(xticks)
    ax1.tick_params(axis='x', labelsize=12)
    ax1.set_xticklabels(groupSizes.tolist() * values, rotation=90)
    ax1.set_xlabel("Group Size")

    ax2 = ax1.twiny()
    ax2.set_xlim(-0.5, values - 0.3)
    ax2.set_xticks(x + width / 2)
    ax2.set_xticklabels(hosts)
    ax2.set_xlabel("Number of Nodes")

def plotTraffic(gs):
    plot_show("traffic_%d" % gs)
    mplot.plotPrepareLogLog(0, 10)

    y = plots[1].get_values_filtered("bandwidth_tx", "groupsize", gs)
    plt.plot(y.x[0], y.sum / 1e6, color=colorsplot[1], alpha=alpha, label="TSS-CoSi", marker="o")

    y = plots[0].get_values_filtered("bw-randhound_tx", "groupsize", gs)
    y2 = plots[0].get_values_filtered("bw-randhound_tx", "groupsize", gs)
    plt.plot(y.x[0], (y.sum + y2.sum) / 1e6, color=colorsplot[0], alpha=alpha, label="RandHound Client", marker="o")

    y = plots[0].get_values_filtered("bandwidth_tx", "groupsize", gs)
    plt.plot(y.x[0], y.sum / 1e6, color=colorsplot[2], alpha=alpha, label="RandHound Total", marker="o")

    plt.legend(loc="lower right")
    plt.ylabel("Bandwidth in MB")
    plt.axes().set_xticks(y.x[0])
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    mplot.plotEnd()


def getWallCPUAvg(stats, column, timeStr, filter_name=None, filter_value=None):
    if filter_name is not None:
        wall = stats.get_values_filtered(column+"_wall", filter_name, filter_value)
        usr = stats.get_values_filtered(column+"_user", filter_name, filter_value)
        sys = stats.get_values_filtered(column+"_system", filter_name, filter_value)
    else:
        wall = stats.get_values(column+"_wall")
        usr = stats.get_values(column+"_user")
        sys = stats.get_values(column+"_system")

    if timeStr.lower() == "cpu":
        return usr.avg + sys.avg
    else:
        return wall.avg

# Colors for the Cothority
# colors = [ '#4183D7','#26A65B', '#F89406', '#CF000F' ]
colorsbar = ["#C5E1C5", "#fffaca", "#ffc2c2", "#c2c2ff"]
colorsplot = [ '#4183D7','#26A65B', '#F89406', '#CF000F' ]

alpha = 0.9
alphabar = 1
mplot = MPlot()


def plot_show(file):
    if write_file:
        mplot.pngname = file


def read_csvs_xname(xname, *values):
    stats = []
    for a in values:
        file = a + '.csv'
        stats.append(CSVStats(file, xname))
    return stats


def read_csvs(*values):
    return read_csvs_xname("hosts", *values)


def read_csv_pure(file, xname, yname):
    ret = {}
    with open(file+'.csv') as csvfile:
        reader = csv.DictReader(csvfile)
        x = 0
        for row in reader:
            if xname != "":
                x = row[xname]
            else:
                x += 1

            ret[x] = float(row[yname])

    return np.array(ret.values())

# Write to file
write_file = True
# file_extension = 'eps'
# Show figure
# mplot.show_fig = True
mplot.show_fig = False

# if False:
# snp17rhound = "snp17_randhound_small"
# snp17tsscosi = "snp17_tsscosi_small"
# snp17cosi = "snp17_cosi_small"
plots = read_csvs("snp17_randhound", "snp17_tsscosi", "snp17_cosi")
for i in range(3):
    for j in [384, 640, 896]:
        plots[i].delete_index_value(j)

snp17rhound, snp17tsscosi, snp17cosi = plots
hosts, groupSizes = snp17rhound.get_limits('groupsize')

# Call all plot-functions
plotRandHerdSetup('Wall')
plotRandHerdSetup('CPU')
plotRandHerdRound()
plotRandHound('Wall')
plotRandHound('CPU')
plotTraffic(groupSizes[-1])
