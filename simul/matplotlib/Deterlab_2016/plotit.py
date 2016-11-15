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

def plotComparison():
    snp17jvss, microcloudjvss = read_csvs("snp17_jvss", "microcloud_jvss")

    plot_show("microcloud_vs_sm")
    mplot.plotPrepareLogLog(0, 0)

    y2 = microcloudjvss.get_values("round_wall")
    plt.plot(y2.x, y2.avg, color=colorsplot[2], alpha=alpha, label="Microcloud", marker="o")

    y1 = snp17jvss.get_values("round_wall")
    plt.plot(y1.x, y1.avg, color=colorsplot[1], alpha=alpha, label="SM", marker="o")

    plt.legend(loc="upper left", prop={'size':legend_size}, title="JVSS running on:")
    plt.ylabel("Wall-clock time (sec)")
    plt.axes().set_xticks([128, 256, 512])
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    # plt.xlim(128/1.1, 1024*1.2)
    mplot.plotEnd()


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

# Call all plot-functions
plotComparison()
