# Plots the graph of one of the test-runs
# It takes the CSV-file as argument and shows the plot
# of the times used for each round

import sys
from mplot import MPlot
from stats import CSVStats
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches


# This one takes two csv-files which represent a Cothority and a JVSS
# run, stacking the user and system-time one upon the other.
def CoJVTimeArea(cothority, jvss):
    mplot.plotPrepareLogLog();
    mplot.plotStacked(jvss, "basic_round", "JVSS system time", "JVSS user time",
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

    ymin = 0.05
    bar_jvss, jvss_sys, jvss_usr = mplot.plotStackedBarsHatched(jvss, "round_system", "round_user", "JVSS", color2_light,
                                                      ymin, delta_x=-1)

    bar_naive, na_sys, na_usr = mplot.plotStackedBarsHatched(naive, "round_system", "round_user", "Naive", color3_light,
                                                     ymin, limit_values=7)

    bar_cothority, co_sys, co_usr = mplot.plotStackedBarsHatched(cothority, "round_system", "round_user", "Cothority",
                                                         color1_light, ymin, delta_x=1, limit_values=11)

    
    ymax = 7
    xmax = 3192
    plt.ylim(ymin, ymax)
    plt.xlim(1.5, xmax)

    usert = mpatches.Patch(color='white', ec='black', label='User time', hatch='//')
    syst = mpatches.Patch(color='white', ec='black', label='System time')

    plt.legend(handles=[bar_jvss, bar_naive, bar_cothority, usert, syst], loc=u'upper left')
    mplot.plotEnd()


# Plots a Cothority and a JVSS run with regard to their averages. Supposes that
# the last two values from JVSS are off-grid and writes them with arrows
# directly on the plot
def plotAvgMM(co, jvss, naive, nt):
    mplot.plotPrepareLogLog()

    #nt = mplot.plotMMA(ntree, 'round_wall', color4_light, 0,
    #                  dict(label='Ntree', linestyle='-', marker='v', color=color4_dark, zorder=3))
    # mplot.arrow("{:.1f} sec      ".format(mplot.avg[-2]), x[-2], 4, 1, color3_dark)
    # mplot.arrow("      {:.0f} sec".format(mplot.avg[-1]), x[-1], 4, 1, color3_dark)

    j = mplot.plotMMA(jvss, 'round_wall', color2_light, 0,
                      dict(label='JVSS', linestyle='-', marker='^', color=color2_dark, zorder=3))
    #j_p = jvss.get_values('round_wall')
    #plt.plot(j_p.x, j_p.avg, label="JVSS", color=color2_dark, marker='^')
    #mplot.arrow("{:.1f} sec      ".format(j.avg[-2]), j.x[-2], 4, 1, color2_dark)
    mplot.arrow("      {:.0f} sec".format(j.avg[-1]), j.x[-1], 4, 1, color2_dark)

    na = mplot.plotMMA(naive, 'round_wall', color3_light, 0,
                       dict(label='Naive', linestyle='-', marker='s', color=color3_dark, zorder=3))
    #na_p = naive.get_values('round_wall')
    mplot.arrow("{:.1f} sec      ".format(na.avg[8]), na.x[8], 4, 1, color3_dark)
    mplot.arrow("      {:.0f} sec".format(na.avg[9]), na.x[9], 4, 1, color3_dark)

    co = mplot.plotMMA(cothority, 'round_wall', color1_light, 4,
                       dict(label='Cothority', linestyle='-', marker='o', color=color1_dark, zorder=5))

    # Make horizontal lines and add arrows for JVSS
    xmin, xmax, ymin, ymax = CSVStats.get_min_max(na, co)
    plt.ylim(ymin, 4)
    plt.xlim(xmin, xmax * 1.2)
    plt.ylabel('Seconds per round')

    plt.legend(loc=u'lower right')
    plt.axes().xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    mplot.plotEnd()


# Plots a Cothority and a JVSS run with regard to their averages. Supposes that
# the last two values from JVSS are off-grid and writes them with arrows
# directly on the plot
def plotAvg(co, jvss, naive, nt):
    mplot.plotPrepareLogLog()

    j_p = jvss.get_values('round_wall')
    plt.plot(j_p.x, j_p.avg, label="JVSS", color=color2_dark, marker='^')

    na_p = naive.get_values('round_wall')
    plt.plot(na_p.x, na_p.avg, label="Naive", color=color3_dark, marker='s')
    #mplot.arrow("      {:.0f} sec".format(na_p.avg[9]), na_p.x[9], 8, 1, color3_dark)

    nt_p = nt.get_values('round_wall')
    plt.plot(nt_p.x, nt_p.avg, label="Ntree", color=color4_dark, marker='v')

    co_p = cothority.get_values('round_wall')
    plt.plot(co_p.x, co_p.avg, label="Cothority", color=color1_dark, marker='o')

    # Make horizontal lines and add arrows for JVSS
    xmin, xmax, ymin, ymax = CSVStats.get_min_max(j_p, na_p, nt_p, co_p)
    plt.ylim(ymin, 8)
    plt.xlim(xmin, 1024 * 1.2)
    plt.ylabel('Seconds per round')

    plt.legend(loc=u'lower right')
    mplot.plotEnd()


def Over(over_1, over_2, over_3):
    mplot.plotPrepareLogLog()

    o3 = mplot.plotMMA(over_3, 'round_wall', color3_light, 0,
                       dict(label='32 Machines', linestyle='-', marker='s', color=color3_dark, zorder=3))

    o2 = mplot.plotMMA(over_2, 'round_wall', color2_light, 0,
                       dict(label='16 Machines', linestyle='-', marker='^', color=color2_dark, zorder=3))

    o1 = mplot.plotMMA(over_1, 'round_wall', color1_light, 0,
                       dict(label='8 Machines', linestyle='-', marker='o', color=color1_dark, zorder=3))

    xmin, xmax, ymin, ymax = CSVStats.get_min_max(o1, o2, o3)
    plt.ylim(ymin, ymax)
    plt.xlim(xmin, xmax * 1.2)
    plt.ylabel('Seconds per round')

    plt.legend(loc=u'lower right')
    mplot.plotEnd()


def PlotMultiBF(*values_bf):
    mplot.plotPrepareLogLog(2, 0)
    plotbf = []
    pparams = [[color1_light, color1_dark, 'o'],
               [color2_light, color2_dark, 's'],
               [color3_light, color3_dark, '^'],
               [color4_light, color4_dark, '.']]

    for i, value in enumerate(values_bf):
        label = str(int(value.columns['Peers'][0])) + " Peers"
        c1, c2, m = pparams[i]
        plotbf.append( mplot.plotMMA(value, 'round_wall', c1, 0,
                           dict(label=label, linestyle='-', marker=m, color=c2, zorder=3)) )

    xmin, xmax, ymin, ymax = CSVStats.get_min_max(*plotbf)
    plt.ylim(ymin, ymax)
    plt.xlim(xmin, xmax * 1.2)
    plt.ylabel('Seconds per round')
    plt.xlabel('Branching factor')

    plt.legend(loc=u'upper right')
    mplot.plotEnd()


# Calculates the time it takes to check the signature
def SigCheck(naive, naive_cs):
    mplot.plotPrepareLogLog()

    # Read in naive
    x = naive.get_values("round_wall").x
    naive_avg = naive.get_values("round_wall").avg
    naive_tsys = naive.get_values("round_system").avg
    naive_tusr = naive.get_values("round_user").avg

    naive_cs_avg = naive_cs.get_values("round_wall").avg
    naive_cs_tsys = naive_cs.get_values("round_system").avg
    naive_cs_tusr = naive_cs.get_values("round_user").avg
    check_avg = [t[0] - t[1] for t in zip(naive_avg, naive_cs_avg)]
    check_tsys = [t[0] - t[1] for t in zip(naive_tsys, naive_cs_tsys)]
    check_tusr = [t[0] - t[1] for t in zip(naive_tusr, naive_cs_tusr)]
    #plt.plot(x, check_avg, label="Round-time", color=color1_dark, marker='o')
    plt.plot(x, check_tsys, label="System time", color=color2_dark, marker='s')
    plt.plot(x, check_tusr, label="User time", color=color3_dark, marker='^')

    plt.legend(loc='upper left')
    plt.ylabel('Time for verification')
    mplot.plotEnd()

def PlotStamp(stamp):
    mplot.plotPrepareLogLog(10, 0)

    plotbf = mplot.plotMMA(stamp, 'round_wall', color1_light, 0,
                           dict(label='4096 Peers', linestyle='-', marker='o', color=color2_dark, zorder=3))

    xmin, xmax, ymin, ymax = CSVStats.get_min_max(plotbf)
    plt.ylim(ymin, ymax)
    plt.xlim(xmin, xmax * 1.2)
    plt.ylabel('Seconds per round')
    plt.xlabel('Stamping rate [1/s]')

    plt.legend(loc=u'upper right')
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
mplot = MPlot()


def check_Args(args):
    if len(sys.argv) < args + 1:
        print("Error: Please give a mode and " + str(args) + " .csv-files as argument - " + str(len(sys.argv)) + "\n")
        print("Mode: (0=printAverage, 1=printSystemUserTimes with bars, 2=printSystemUserTimes with areas)\n")
        print("CSV: cothority.csv jvss.csv\n")
        exit(1)


def plot_show(argn):
    if len(sys.argv) > 2 + argn:
        mplot.pngname = sys.argv[2 + argn]
        mplot.show_fig = False
    print mplot.pngname, mplot.show_fig


def args_to_csv(argn, xname = "Peers"):
    stats = []
    for a in sys.argv[2:argn + 2]:
        stats.append(CSVStats(a, xname))
    plot_show(argn)
    return stats


option = sys.argv[1]

if option == "0":
    cothority, jvss, naive, naive_sc, ntree = args_to_csv(5)
    plotAvgMM(cothority, jvss, naive, ntree)
elif option == "1":
    cothority, jvss, naive, naive_sc, ntree = args_to_csv(5)
    CoJVTimeBars(cothority, jvss, naive)
elif option == "2":
    cothority, jvss, naive, naive_sc, ntree = args_to_csv(5)
    CoJVTimeArea(cothority, jvss)
elif option == "3":
    cothority, jvss, naive, naive_sc, ntree = args_to_csv(5)
    SigCheck(naive, naive_sc)
elif option == "4":
    Over(*args_to_csv(3))
elif option == "5":
    PlotMultiBF(*args_to_csv(1, "bf"))
elif option == "6":
    plot_show(1)
    stamp = CSVStats(sys.argv[2], "rate")
    PlotStamp(stamp)
elif option == "7":
    PlotMultiBF(*args_to_csv(4, "bf"))

