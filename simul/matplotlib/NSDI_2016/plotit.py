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
            mplot.plotMMA(plots[index], csv_column, colors[index][0], 4,
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

def plotVerify():
    plotData(
        [['swup_verification'],
         ['PGP key verification']],
        'swup_verification',
        loglog=[10, 10],
        xname="pgpkeys",
        csv_column="verification_wall",
        xlabel="Number of PGP-signatures on release",
        ylabel="Time to verify PGP-signatures [s]"
    )

def plotFull():
    plotData(
        [['test_swupcreate'],
         ['Software-update']],
        'full_ls',
        csv_column="full_ls_wall"
    )

def plotSBCreation():
    plots = read_csvs("swup_create")[0]
    plot_show("swup_create")
    mplot.plotPrepareLogLog(0, 10)
    width = 0.25
    x = np.arange(len(plots.x))
    yb = np.zeros(len(x))
    vls = [['verification_wall', 'PGP Signature verification'],
           ['add_block_wall', 'Collective signing'],
           ['build_wall', 'Reproducible build']]
    handles = []
    labels = []
    for index, vl in enumerate(vls):
        value, label = vl
        y = np.array(plots.get_values(value).avg)
        if value == 'build_wall':
            y = np.ones(len(x)) * y[0]
        h = plt.bar(x+(index-1)*width, y, width, label=label, color=colors[index][0])
        # h = plt.bar(x, y, width, bottom=yb, label=label, color=colors[index][0])
        handles.append(h)
        labels.append(label)
        yb += y

    total = np.array(plots.get_values('overall_nobuild_wall').avg) + y
    oa = plt.plot(x+width / 2, total, color=colors[len(vls)][1], marker='x')
    labels.append("Total for new package")
    plt.xticks(x + width / 2, plots.x)
    plt.legend(oa + handles[::-1], labels[::-1], loc='lower right')
    plt.ylim(0.001, 500)
    plt.xlim(x[0]-width, x[-1]+2*width)
    mplot.plotEnd()

def plotBuildCDF():
    files = [
        # ['repro_builds_essential', 'Debian essential packages'],
             ['repro_builds_required', 'Debian required packages'],
             ['repro_builds_random', 'Random set of 50 Debian packages'],
             ['repro_builds_popular', '50 most popular Debian packages'],
             ]

    mplot.plotPrepareLogLog(0, 0)
    plot_show("repro_build_cdf")

    for index, fl in enumerate(files):
        file, label = fl
        values = read_csv_pure(file, "", "wall_time").values()
        X = np.sort(np.array(values))
        while X[0] < 100.0:
            X = np.delete(X, 0)
        X /= 60
        Y = np.linspace(0, 100, len(X))

        plt.plot(X, Y, label=label, linestyle='-', marker='o',
                 color=colors[index][1])

    plt.xlabel("Time [minutes]")
    plt.ylabel("% of Packages Built")

    plt.legend(loc='lower right')
    ax = plt.axes()
    ax.xaxis.grid(color='gray', linestyle='dashed', zorder=0)
    ax.grid(which='both')
    ax.grid(which='minor', alpha=0.2)
    ax.grid(which='major', alpha=0.5)
    plt.xticks(np.linspace(0,10,11))
    plt.yticks(np.linspace(0,100,11))
    plt.xlim(1,10)
    plt.annotate("Aptitude: 13'", xy=(10, 95),
                 xytext=(8, 95),
                 # horizontalalignment='left',
                 verticalalignment='top',
                 arrowprops=dict(facecolor=colors[2][1], ec=colors[2][1],
                                 headlength=5, width=0.1,
                                 headwidth=8))
    plt.annotate("Perl: 40'", xy=(10, 90),
                 xytext=(8, 90),
                 # horizontalalignment='left',
                 verticalalignment='top',
                 arrowprops=dict(facecolor=colors[0][1], ec=colors[0][1],
                                 headlength=5, width=0.1,
                                 headwidth=8))
    mplot.plotEnd()


def plotBW3():
    plotData(
        [['swup_random_update_11_4', 'swup_random_update_5_7', 'swup_random_update_1_1'],
         ['Linear skipchain', 'S5_7 skipchain', 'S11_4 skipchain']],
        'update_bandwidth',
        loglog=[10, 10],
        xname="frequency",
        csv_column="client_bw_swupdate_rx",
        xlabel="Days between two updates",
        ylabel="Bandwidth for 400 days of updates"
    )

def plotBW():
    mplot.plotPrepareLogLog(2, 10)
    plot_show('update_bandwidth')
    for index, sparam in enumerate(['1_1', '2_5', '3_5', '4_5']):
        data = read_csvs_xname('frequency','swup_random_update_' + sparam)[0]
        bandwidth = np.array(data.columns['client_bw_swupdate_tx_sum']) + \
                    np.array(data.columns['client_bw_swupdate_rx_sum'])
        print bandwidth
        plt.plot(data.x, bandwidth, label="SkipChain S" + sparam)

    plt.xlabel("Client update frequency")
    plt.xticks(data.x)
    plt.ylabel("Bandwidth for one update of the client")
    plt.legend(loc='lower left')
    mplot.plotEnd()


# Colors for the Cothority
colors = [['lightgreen', 'green'],
          ['lightblue', 'blue'],
          ['yellow', 'brown'],
          ['pink', 'red'],
          ['pink', 'red']]
mplot = MPlot()


def plot_show(file):
    if write_file:
        mplot.pngname = file
        # mplot.pngname = file + '.' + file_extension


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

    return ret

# Write to file
write_file = True
# What file extension - .png, .eps
file_extension = 'png'
# file_extension = 'eps'
# Show figure
mplot.show_fig = True
mplot.show_fig = False

# Call all plot-functions
# plotFull()
# plotVerify()
# plotSBCreation()
# plotBuildCDF()
plotBW()