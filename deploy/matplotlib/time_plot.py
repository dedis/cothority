# Plots the graph of one of the test-runs
# It takes the CSV-file as argument and shows the plot
# of the times used for each round

import matplotlib.pyplot as plt
import matplotlib.ticker
import csv
import sys
import math
import matplotlib.patches as mpatches
from matplotlib.legend_handler import HandlerLine2D, HandlerRegularPolyCollection
from mplot import MPlot


# This one takes two csv-files which represent a Cothority and a JVSS
# run, stacking the user and system-time one upon the other.
def CoJVTimeArea(cothority, jvss):
    mplot.plotPrepareLogLog();
    mplot.xmin = -1
    mplot.readCSV(jvss)
    mplot.plotStacked(mplot.x, mplot.tsys, mplot.tusr, "JVSS system time", "JVSS user time",
                color2_light, color2_dark)
    mm = [min(mplot.tsys), max(mplot.tusr)]

    mplot.readCSV(cothority)
    mplot.plotStacked(mplot.x, mplot.tsys, mplot.tusr, "Cothority system time", "Cothority user time",
                color1_light, color1_dark, min(mm))
    mm = [min(mm[0], min(mplot.tsys)), max(mm[1], max(mplot.tusr))]

    plt.ylim(min(mplot.tsys), mm[1])
    plt.xlim(mplot.xmin, mplot.xmax * 1.3)
    plt.legend()
    mplot.plotEnd()


# This one takes two csv-files which represent a Cothority and a JVSS
# run, stacking the user and system-time one upon the other.
def CoJVTimeBars(cothority, jvss, naive):
    mplot.plotPrepareLogLog();
    mplot.xmin = -1

    mplot.readCSV(jvss)
    bar_jvss = mplot.plotStackedBarsHatched(mplot.x, mplot.tsys, mplot.tusr, "JVSS", color2_light, delta_x=-1)
    mm = [min(mplot.tsys), max(mplot.tusr)]

    mplot.readCSV(naive)
    bar_naive = mplot.plotStackedBarsHatched(mplot.x, mplot.tsys, mplot.tusr, "Naive", color3_light)
    mm = [min(mplot.tsys), max(mplot.tusr)]

    mplot.readCSV(cothority)
    bar_cothority = mplot.plotStackedBarsHatched(mplot.x, mplot.tsys, mplot.tusr, "Cothority", color1_light, min(mm), delta_x=1)
    mm = [min(mm[0], min(mplot.tsys)), max(mm[1], max(mplot.tusr))]

    plt.ylim(min(mplot.tsys), mm[1] * 4)
    plt.xlim(mplot.xmin, mplot.xmax * 1.3)

    usert = mpatches.Patch(color='white', ec='black', label='User time', hatch='//')
    syst = mpatches.Patch(color='white', ec='black', label='System time')

    plt.legend(handles=[bar_jvss, bar_naive, bar_cothority, usert, syst], loc = u'upper left')
    mplot.plotEnd()



# Plots a Cothority and a JVSS run with regard to their averages. Supposes that
# the last two values from JVSS are off-grid and writes them with arrows
# directly on the plot
def plotAvg(cothority, jvss, naive, ntree):
    mplot.plotPrepareLogLog()

    mplot.xmin = -1
    mplot.readCSV(jvss)
    plt.plot(mplot.x, mplot.avg, label='JVSS', linestyle='-', marker='^', color=color2_dark, zorder=3)
    mplot.plotFilledLegend(mplot.x, mplot.tmin, mplot.tmax, "min-max", color2_light, z=0)
    mplot.arrow("{:.1f} sec      ".format(mplot.avg[-2]), mplot.x[-2], 4, color2_dark)
    mplot.arrow("      {:.0f} sec".format(mplot.avg[-1]), mplot.x[-1], 4, color2_dark)

    mplot.readCSV(naive)
    plt.plot(mplot.x, mplot.avg, label='Naive', linestyle='-', marker='s', color=color3_dark, zorder=3)
    mplot.plotFilledLegend(mplot.x, mplot.tmin, mplot.tmax, "min-max", color3_light, z=0)
    # arrow("{:.1f} sec      ".format(avg[-2]), x[-2], 4, color3_dark)
    mplot.arrow("      {:.0f} sec".format(mplot.avg[-1]), mplot.x[-1], 4, color3_dark)

    mplot.readCSV(ntree)
    plt.plot(mplot.x, mplot.avg, label='Ntree', linestyle='-', marker='s', color=color4_dark, zorder=3)
    mplot.plotFilledLegend(mplot.x, mplot.tmin, mplot.tmax, "min-max", color4_light, z=0)
    # arrow("{:.1f} sec      ".format(avg[-2]), x[-2], 4, color3_dark)
    #arrow("      {:.0f} sec".format(avg[-1]), x[-1], 4, color3_dark)

    mplot.readCSV(cothority)
    plt.plot(mplot.x, mplot.avg, label='Cothority', linestyle='-', marker='o', color=color1_dark, zorder=5)
    mplot.plotFilledLegend(mplot.x, mplot.tmin, mplot.tmax, "min-max", color1_light, z=4)

    # Make horizontal lines and add arrows for JVSS
    plt.ylim(mplot.ymin, 4)
    plt.xlim(mplot.xmin, mplot.xmax * 1.2)
    plt.ylabel('Seconds per round')

    plt.legend(loc=u'lower right')
    mplot.plotEnd()


# Calculates the time it takes to check the signature
def SigCheck(naive, naive_cs):
    mplot.plotPrepareLogLog()
    xmin = -1

    # Read in naive
    mplot.readCSV(naive)
    naive_avg = mplot.avg
    naive_tsys = mplot.tsys
    naive_tusr = mplot.tusr

    mplot.readCSV(naive_cs)
    check_avg = [t[0] - t[1] for t in zip(naive_avg, mplot.avg)]
    check_tsys = [t[0] - t[1] for t in zip(naive_tsys, mplot.tsys)]
    check_tusr = [t[0] - t[1] for t in zip(naive_tusr, mplot.tusr)]
    plt.plot(mplot.x, check_avg, label="Round-time", color=color1_dark, marker='o')
    plt.plot(mplot.x, check_tsys, label="System time", color=color2_dark, marker='s')
    plt.plot(mplot.x, check_tusr, label="User time", color=color3_dark, marker='^')

    plt.legend(loc='upper left')
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
args = 5

mplot = MPlot()

if len(sys.argv) < args + 1:
    print("Error: Please give a mode and " + str(args) + " .csv-files as argument - "+str(len(sys.argv))+"\n")
    print("Mode: (0=printAverage, 1=printSystemUserTimes with bars, 2=printSystemUserTimes with areas)\n")
    print("CSV: cothority.csv jvss.csv\n")
    exit(1)

show_fig = True
if len(sys.argv) > args + 2:
    show_fig = False
    pngname = sys.argv[-1]

option = sys.argv[1]
cothority, jvss, naive, naive_sc, ntree = sys.argv[2:args+2]
if option == "0":
    plotAvg(cothority, jvss, naive, ntree)
elif option == "1":
    CoJVTimeBars(cothority, jvss, naive)
elif option == "2":
    CoJVTimeArea(cothority, jvss)
elif option == "3":
    SigCheck(naive, naive_sc)
