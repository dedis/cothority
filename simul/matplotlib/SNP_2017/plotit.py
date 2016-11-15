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
import numpy as np

import csv

def plotRandHerdSetup(timeStr):
    plot_show("randherd_setup_" + timeStr)
    mplot.plotPrepareLogLog(0, 10)
    width = 0.2
    values = len(hosts)
    x = np.arange(values)
    handles = []
    labels = ['RandHound', 'TSS-Key-Setup', 'CoSi']
    for index, groupSize in enumerate(groupSizes):
        cosi = getWallCPUAvg(plots[2], 'round', timeStr)

        # rhound = getWallCPUAvg(plots[0], 'tgen-randhound', timeStr, 'groupsize', groupSize)
        rhound = getWallCPUAvg(plots[0], 'randhound_server_i1', timeStr, 'groupsize', groupSize)
        rhound += getWallCPUAvg(plots[0], 'randhound_server_i2', timeStr, 'groupsize', groupSize)
        rhound *= hosts
        rhound += getWallCPUAvg(plots[0], 'tgen-randhound', timeStr, 'groupsize', groupSize)
        rhoundver = getWallCPUAvg(plots[0], 'tver-randhound', timeStr, 'groupsize', groupSize)

        tsscosi = getWallCPUAvg(plots[1], 'setup', timeStr, 'groupsize', groupSize)

        if timeStr.lower() == "cpu":
            cosi *= hosts
            tsscosi *= hosts
            rhoundver *= hosts

        rhound += rhoundver

        cosi /= 3600
        rhound /= 3600
        tsscosi /= 3600

        h1 = plt.bar(x+(index-1.5)*width, cosi, width, color=colorsbar[0], alpha=alphabar)
        h2 = plt.bar(x+(index-1.5)*width, tsscosi, width, color=colorsbar[1], alpha=alphabar,
                bottom=cosi)
        h3 = plt.bar(x+(index-1.5)*width, rhound, width, color=colorsbar[2], alpha=alphabar,
                bottom=cosi+tsscosi)

        if index == 0:
            handles = [h3, h2, h1]
            ymin = cosi[0]

        if index == len(groupSizes) - 1:
            lastx = values - 1
            ymax = cosi[lastx] + tsscosi[lastx] + rhound[lastx]

    plt.legend(handles , labels, loc='upper left', prop={'size':legend_size})
    if timeStr.lower() == "cpu":
        plt.ylabel("CPU-Usage (hours)")
    else:
        plt.ylabel(timeStr + "-Time for RandHerd Setup (hours)")
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
        plt.plot(y.x[0], y.min + 2, color=colorsplot[index], alpha=alpha, marker="o",
                 label="%d" % groupSize)
        # y1 = plot.get_values_filtered("round_user", "groupsize", groupSize)
        # y2 = plot.get_values_filtered("round_system", "groupsize", groupSize)
        # plt.plot(y1.x[0], (y1.min + y2.min), color=colorsplot[index], alpha=alpha, marker="x",
        #          label="%d" % groupSize)
    plt.axes().set_xticks(hosts)
    plt.legend(loc="upper left", title="Group Size", ncol=2, prop={'size':legend_size})
    #plt.ylabel("Wall-clock Time for one RandHerd Round (sec)")
    plt.ylabel("Wall-clock Time (sec)")
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
    labels = ['Verification (Client)', 'Generation (Client)', 'Generation (All Servers)']
    for index, groupSize in enumerate(groupSizes):
        server = getWallCPUAvg(plot, 'randhound_server_i1', timeStr, 'groupsize', groupSize)
        server += getWallCPUAvg(plot, 'randhound_server_i2', timeStr, 'groupsize', groupSize)
        if timeStr.lower() == "cpu":
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

    if timeStr.lower() == "cpu":
        plt.legend(handles, labels, loc='upper left', prop={'size':legend_size})
        #plt.ylabel("CPU-Usage for the Complete System (sec)")
        plt.ylabel("CPU-Usage (sec)")
    else:
        plt.legend(handles, labels[:2], loc='upper left', prop={'size':legend_size})
        plt.ylabel("Wall-time (sec)")
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

    y = plots_traffic[1].get_values_filtered("bandwidth_tx", "groupsize", gs)
    plt.plot(y.x[0], y.sum / 1e6, color=colorsplot[1], alpha=alpha,
            label="Randherd", marker="o")

    y = plots_traffic[0].get_values_filtered("bandwidth_tx", "groupsize", gs)
    plt.plot(y.x[0], y.sum / 1e6, color=colorsplot[0], alpha=alpha, label="RandHound", marker="o")

    y = plots_traffic[2].get_values("bandwidth_tx")
    plt.plot(y.x, y.sum / 1e6, color=colorsplot[2], alpha=alpha, label="CoSi", marker="o")

    plt.legend(loc="lower right", prop={'size':legend_size})
    plt.ylabel("Communication Cost (MByte)")
    plt.axes().set_xticks(y.x)
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    mplot.plotEnd()

def plotBaseline(gs):
    plot_show("baseline_%d" % gs)
    mplot.plotPrepareLogLog(10, 10)

    y3 = jvss.get_values("round_wall")
    plt.plot(y3.x, y3.avg, color=colorsplot[2], alpha=alpha, label="RandShare", marker="o")

    y2 = snp17rhound.get_values_filtered("tgen-randhound_wall", "groupsize", gs)
    plt.plot(y2.x[0], y2.avg, color=colorsplot[1], alpha=alpha, label="RandHound (GroupSize=%d)" % gs, marker="o")

    y1 = snp17tsscosi.get_values_filtered("round_wall", "groupsize", gs)
    plt.plot(y1.x[0], y1.avg, color=colorsplot[0], alpha=alpha, label="RandHerd (GroupSize=%d)" % gs, marker="o")

    plt.legend(loc="upper left", prop={'size':legend_size})
    plt.ylabel("Wall-clock time (sec)")
    plt.axes().set_xticks([128, 256, 512, 768, 1024])
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    plt.xlim(128/1.1, 1024*1.2)
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
colorsbar = ["#c2c2ff", "#C5E1C5", "#fffaca", "#ffc2c2"]
colorsplot = [ '#4183D7','#26A65B', '#F89406', '#CF000F' ]

alpha = 0.9
alphabar = 1
mplot = MPlot()
legend_size = 12


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
# Show figure
# mplot.show_fig = True
mplot.show_fig = False

plots = read_csvs("snp17_randhound", "snp17_tsscosi", "snp17_cosi", "snp17_jvss")
plots_traffic = read_csvs("snp17_randhound_traffic", "snp17_tsscosi_traffic",
                          "snp17_cosi_traffic")

snp17rhound, snp17tsscosi, snp17cosi, jvss = plots
hosts, groupSizes = snp17rhound.get_limits('groupsize')

# Call all plot-functions
plotRandHerdSetup('Wall')
plotRandHerdSetup('CPU')
plotRandHerdRound()
plotRandHound('Wall')
plotRandHound('CPU')
plotTraffic(32)
plotBaseline(32)

# plotTraffic(groupSizes[-1])
